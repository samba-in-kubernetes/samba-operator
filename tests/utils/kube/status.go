package kube

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/poll"
)

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

type podProbe struct {
	poll.Prober

	tc        *TestClient
	fetchOpts PodFetchOptions
	pods      []corev1.Pod
}

func (pp *podProbe) fetch(ctx context.Context) error {
	pods, err := pp.tc.FetchPods(ctx, pp.fetchOpts)
	if err == nil {
		pp.pods = pods
	}
	return err
}

func (pp *podProbe) checkExists(ctx context.Context) (bool, error) {
	err := pp.fetch(ctx)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrNoMatchingPods) {
		return false, nil
	}
	if errors.Is(err, ErrTooFewMatchingPods) {
		return false, nil
	}
	return false, err
}

func (pp *podProbe) checkReady(ctx context.Context) (bool, error) {
	err := pp.fetch(ctx)
	if err != nil {
		return false, err
	}
	for _, pod := range pp.pods {
		if PodIsReady(&pod) {
			return true, nil
		}
	}
	return false, nil
}

func (pp *podProbe) checkAllReady(ctx context.Context) (bool, error) {
	err := pp.fetch(ctx)
	if err != nil {
		return false, err
	}
	for _, pod := range pp.pods {
		if !PodIsReady(&pod) {
			return false, nil
		}
	}
	return true, nil
}

func (pp *podProbe) Completed(e error) error {
	if e == nil {
		return e
	}
	return fmt.Errorf(
		"%w (opts: %+v; pods: %+v)",
		e,
		pp.fetchOpts,
		podsSummary(pp.pods),
	)
}

func podsSummary(pods []corev1.Pod) []string {
	s := []string{}
	for _, p := range pods {
		s = append(s, podSummary(p))
	}
	return s
}

func podSummary(pod corev1.Pod) string {
	return fmt.Sprintf("pod(%s/%s, ready=%v)",
		pod.Namespace,
		pod.Name,
		PodIsReady(&pod),
	)
}

// WaitForAnyPodReady will wait for a pod to be ready, up to the deadline
// specified by the context, if the context lacks a deadline the call will
// block indefinitely. Pods are selected using the PodFetchOptions.
func WaitForAnyPodReady(
	ctx context.Context, tc *TestClient, fo PodFetchOptions) error {
	// ---
	pp := &podProbe{
		tc:        tc,
		fetchOpts: fo,
	}
	pp.Prober.Cond = func() (bool, error) { return pp.checkReady(ctx) }
	return poll.TryUntil(ctx, pp)
}

// WaitForAllPodReady will for all pods selected to be ready, up to the
// deadline specified by the context, if the context lacks a deadline the
// call will block indefinitely. Pods are selected using the PodFetchOptions.
func WaitForAllPodReady(
	ctx context.Context, tc *TestClient, fo PodFetchOptions) error {
	// ---
	pp := &podProbe{
		tc:        tc,
		fetchOpts: fo,
	}
	pp.Prober.Cond = func() (bool, error) { return pp.checkAllReady(ctx) }
	return poll.TryUntil(ctx, pp)
}

// WaitForAnyPodExists will wait for a pod to exist, up to the deadline
// specified by the context, if the context lacks a deadline the call will
// block indefinitely. Pods are selected using the PodFetchOptions.
func WaitForAnyPodExists(
	ctx context.Context, tc *TestClient, fo PodFetchOptions) error {
	// ---
	pp := &podProbe{
		tc:        tc,
		fetchOpts: fo,
	}
	pp.Prober.Cond = func() (bool, error) { return pp.checkExists(ctx) }
	return poll.TryUntil(ctx, pp)
}
