package kube

import (
	"context"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
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
			} else if errors.Is(err, ErrTooFewMatchingPods) {
				return false, nil
			}
			return false, err
		},
	)
}
