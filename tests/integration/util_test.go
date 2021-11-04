// +build integration

package integration

import (
	"context"
	"time"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
)

var (
	waitForPodsTime  = 20 * time.Second
	waitForReadyTime = 60 * time.Second
)

type checker interface {
	NoError(err error, m ...interface{})
}

func createFromFiles(
	require checker, tc *kube.TestClient, srcs []kube.FileSource) {
	// ---
	for _, fs := range srcs {
		_, err := tc.CreateFromFileIfMissing(
			context.TODO(),
			fs,
		)
		require.NoError(err)
	}
}

func deleteFromFiles(
	require checker, tc *kube.TestClient, srcs []kube.FileSource) {
	// ---
	for _, fs := range srcs {
		err := tc.DeleteResourceMatchingFile(
			context.TODO(),
			fs,
		)
		require.NoError(err)
	}
}

type withClient interface {
	getTestClient() *kube.TestClient
}

type podTestClient interface {
	withClient
	getPodFetchOptions() kube.PodFetchOptions
}

func waitForPodExist(s podTestClient) error {
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(waitForPodsTime))
	defer cancel()
	return kube.WaitForAnyPodExists(
		ctx,
		s.getTestClient(),
		s.getPodFetchOptions(),
	)
}

func waitForPodReady(s podTestClient) error {
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(waitForReadyTime))
	defer cancel()
	return kube.WaitForAnyPodReady(
		ctx,
		s.getTestClient(),
		s.getPodFetchOptions(),
	)
}
