// +build integration

package integration

import (
	"context"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
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
