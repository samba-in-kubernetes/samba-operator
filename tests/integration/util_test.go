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

type TestContextKey string

const (
	TestIDKey    = TestContextKey("testID")
	TestShareKey = TestContextKey("testShare")
)

var (
	waitForIpTime    = 120 * time.Second
	waitForPodsTime  = 120 * time.Second
	waitForReadyTime = 200 * time.Second
	waitForClearTime = 200 * time.Millisecond
	clientCreateTime = 120 * time.Second
)

type checker interface {
	NoError(err error, m ...interface{})
}

func testContext() context.Context {
	return context.Background()
}

func createFromFiles(
	ctx context.Context,
	require checker,
	tc *kube.TestClient,
	srcs []kube.FileSource) {
	// ---
	for _, fs := range srcs {
		_, err := tc.CreateFromFileIfMissing(
			ctx,
			fs,
		)
		require.NoError(err)
	}
}

func deleteFromFiles(
	ctx context.Context,
	require checker,
	tc *kube.TestClient,
	srcs []kube.FileSource) {
	// ---
	for _, fs := range srcs {
		err := tc.DeleteResourceMatchingFile(
			ctx,
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

func waitForPodExist(ctx context.Context, s podTestClient) error {
	ctx, cancel := context.WithDeadline(
		ctx,
		time.Now().Add(waitForPodsTime))
	defer cancel()
	return kube.WaitForAnyPodExists(
		ctx,
		s.getTestClient(),
		s.getPodFetchOptions(),
	)
}

func waitForPodReady(ctx context.Context, s podTestClient) error {
	ctx, cancel := context.WithDeadline(
		ctx,
		time.Now().Add(waitForReadyTime))
	defer cancel()
	return kube.WaitForAnyPodReady(
		ctx,
		s.getTestClient(),
		s.getPodFetchOptions(),
	)
}

func waitForAllPodReady(ctx context.Context, s podTestClient) error {
	ctx, cancel := context.WithDeadline(
		ctx,
		time.Now().Add(waitForReadyTime))
	defer cancel()
	return kube.WaitForAllPodReady(
		ctx,
		s.getTestClient(),
		s.getPodFetchOptions(),
	)
}

func createSMBClientIfMissing(
	ctx context.Context, require *require.Assertions, tc *kube.TestClient) {
	// ---
	_, err := tc.CreateFromFileIfMissing(
		ctx,
		kube.FileSource{
			Path:      path.Join(testFilesDir, "data1.yaml"),
			Namespace: testNamespace,
		})
	require.NoError(err)

	_, err = tc.CreateFromFileIfMissing(
		ctx,
		kube.FileSource{
			Path:      path.Join(testFilesDir, "client-test-pod.yaml"),
			Namespace: testNamespace,
		})
	require.NoError(err)

	// ensure the smbclient test pod exists and is ready
	ctx2, cancel := context.WithTimeout(
		ctx,
		clientCreateTime)
	defer cancel()
	l := "app=samba-operator-test-smbclient"
	require.NoError(kube.WaitForAnyPodExists(
		ctx2,
		kube.NewTestClient(""),
		kube.PodFetchOptions{
			Namespace:     testNamespace,
			LabelSelector: l,
		}),
		"smbclient pod does not exist",
	)
	require.NoError(kube.WaitForAnyPodReady(
		ctx2,
		kube.NewTestClient(""),
		kube.PodFetchOptions{
			Namespace:     testNamespace,
			LabelSelector: l,
		}),
		"smbclient pod not ready",
	)
}
