package kube

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	// ErrNoMatchingPods indicates a selector didn't match any pods.
	ErrNoMatchingPods = errors.New("no pods match selector")

	// ErrTooManyMatchingPods indicates a selector should have matched
	// fewer pods than were selected.
	ErrTooManyMatchingPods = errors.New("too many pods match selector")

	// ErrTooFewMatchingPods indicates a selector should have matched
	// more pods than were selected.
	ErrTooFewMatchingPods = errors.New("too few pods match selector")
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
	MinFound      int
}

func (o PodFetchOptions) max() int {
	if o.MaxFound == 0 {
		return 1
	}
	return o.MaxFound
}

func (o PodFetchOptions) min() int {
	return o.MinFound
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
		return nil, ErrTooManyMatchingPods
	}
	if len(l.Items) == 0 {
		return nil, ErrNoMatchingPods
	}
	if len(l.Items) < fo.min() {
		return nil, ErrTooFewMatchingPods
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
