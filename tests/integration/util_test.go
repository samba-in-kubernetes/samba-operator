//go:build integration
// +build integration

package integration

import (
	"context"
	"path"
	"time"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/stretchr/testify/require"
)

var (
	waitForIpTime    = 120 * time.Second
	waitForPodsTime  = 120 * time.Second
	waitForReadyTime = 200 * time.Second
	waitForClearTime = 200 * time.Millisecond
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

func waitForAllPodReady(s podTestClient) error {
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(waitForReadyTime))
	defer cancel()
	return kube.WaitForAllPodReady(
		ctx,
		s.getTestClient(),
		s.getPodFetchOptions(),
	)
}

func createSMBClientIfMissing(require *require.Assertions, tc *kube.TestClient) {
	// ---
	_, err := tc.CreateFromFileIfMissing(
		context.TODO(),
		kube.FileSource{
			Path:      path.Join(testFilesDir, "data1.yaml"),
			Namespace: testNamespace,
		})
	require.NoError(err)

	_, err = tc.CreateFromFileIfMissing(
		context.TODO(),
		kube.FileSource{
			Path:      path.Join(testFilesDir, "client-test-pod.yaml"),
			Namespace: testNamespace,
		})
	require.NoError(err)

	// ensure the smbclient test pod exists and is ready
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(120*time.Second))
	defer cancel()
	l := "app=samba-operator-test-smbclient"
	require.NoError(kube.WaitForAnyPodExists(
		ctx,
		kube.NewTestClient(""),
		kube.PodFetchOptions{
			Namespace:     testNamespace,
			LabelSelector: l,
		}),
		"smbclient pod does not exist",
	)
	require.NoError(kube.WaitForAnyPodReady(
		ctx,
		kube.NewTestClient(""),
		kube.PodFetchOptions{
			Namespace:     testNamespace,
			LabelSelector: l,
		}),
		"smbclient pod not ready",
	)
}
