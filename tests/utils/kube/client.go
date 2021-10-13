package kube

import (
	"context"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	// ErrNoMatchingPods indicates a selector didn't match any pods.
	ErrNoMatchingPods = errors.New("no pods match selector")

	// ErrMultipleMatchingPods indicates a selector should have matched
	// fewer pods than were selected.
	ErrMultipleMatchingPods = errors.New("too many pods match selector")
)

// TestClient is a helper for doing common things for our tests
// easily in kubernetes. This aims to help write integration tests.
type TestClient struct {
	cfg       *rest.Config
	clientset *kubernetes.Clientset
}

// Clientset returns the exact clientset used for this client.
func (tc *TestClient) Clientset() *kubernetes.Clientset {
	return tc.clientset
}

// PodFetchOptions controls what set of pods will be fetched.
type PodFetchOptions struct {
	Namespace     string
	LabelSelector string
	MaxFound      int
}

func (o PodFetchOptions) max() int {
	if o.MaxFound == 0 {
		return 1
	}
	return o.MaxFound
}

// FetchPods returns all available pods matching the PodFetchOptions.
func (tc *TestClient) FetchPods(
	ctx context.Context, fo PodFetchOptions) ([]corev1.Pod, error) {
	// ---
	opts := metav1.ListOptions{
		LabelSelector: fo.LabelSelector,
	}
	l, err := tc.Clientset().CoreV1().Pods(fo.Namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	if len(l.Items) > fo.max() {
		return nil, ErrMultipleMatchingPods
	}
	if len(l.Items) == 0 {
		return nil, ErrNoMatchingPods
	}
	return l.Items, nil
}

// NewTestClient return a new kube util test client.
func NewTestClient(kubeconfig string) *TestClient {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}
	kcc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, &clientcmd.ConfigOverrides{})
	// TODO: add (or just verify) ability to also run _inside_ k8s
	// rather than on some external node
	tc := &TestClient{}
	var err error
	tc.cfg, err = kcc.ClientConfig()
	if err != nil {
		panic(err)
	}
	tc.clientset = kubernetes.NewForConfigOrDie(tc.cfg)
	return tc
}

// PodIsReady returns true if a pod is running and containers are ready.
func PodIsReady(pod *corev1.Pod) bool {
	var podReady, containersReady bool
	if pod.Status.Phase == corev1.PodRunning {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady {
				podReady = cond.Status == corev1.ConditionTrue
			} else if cond.Type == corev1.ContainersReady {
				containersReady = cond.Status == corev1.ConditionTrue
			}
		}
	}
	return podReady && containersReady
}

func waitFor(ctx context.Context, cond func() (bool, error)) error {
	for {
		c, err := cond()
		if err != nil {
			return err
		}
		if c {
			break
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

// WaitForAnyPodReady will wait for a pod to be ready, up to the deadline
// specified by the context, if the context lacks a deadline the call will
// block indefinitely. Pods are selected using the PodFetchOptions.
func WaitForAnyPodReady(
	ctx context.Context, tc *TestClient, fo PodFetchOptions) error {
	// ---
	return waitFor(
		ctx,
		func() (bool, error) {
			pods, err := tc.FetchPods(ctx, fo)
			if err != nil {
				return false, err
			}
			for _, pod := range pods {
				if PodIsReady(&pod) {
					return true, nil
				}
			}
			return false, nil
		},
	)
}

// WaitForAnyPodExists will wait for a pod to exist, up to the deadline
// specified by the context, if the context lacks a deadline the call will
// block indefinitely. Pods are selected using the PodFetchOptions.
func WaitForAnyPodExists(
	ctx context.Context, tc *TestClient, fo PodFetchOptions) error {
	// ---
	return waitFor(
		ctx,
		func() (bool, error) {
			_, err := tc.FetchPods(ctx, fo)
			if err == nil {
				return true, nil
			} else if errors.Is(err, ErrNoMatchingPods) {
				return false, nil
			}
			return false, err
		},
	)
}
