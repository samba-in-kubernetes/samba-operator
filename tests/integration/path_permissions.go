// +build integration

package integration

import (
	"context"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/types"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
)

type MountPathPermissionsSuite struct {
	suite.Suite

	commonSources      []kube.FileSource
	smbshareSources    []kube.FileSource
	smbShareResource   types.NamespacedName
	serverLabelPattern string
	tc                 *kube.TestClient
}

func (s *MountPathPermissionsSuite) waitForPods(labelPattern string) {
	require := s.Require()
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(waitForPodsTime))
	defer cancel()
	opts := kube.PodFetchOptions{
		Namespace:     testNamespace,
		LabelSelector: labelPattern,
	}
	require.NoError(
		kube.WaitForAnyPodExists(ctx, kube.NewTestClient(""), opts),
		"pod does not exist",
	)
	require.NoError(
		kube.WaitForAnyPodReady(ctx, kube.NewTestClient(""), opts),
		"pod not ready",
	)
}

func (s *MountPathPermissionsSuite) SetupSuite() {
	s.tc = kube.NewTestClient("")
	require := s.Require()

	// Create smbshare with Spec.Storage.PVC.Path specified
	createFromFiles(context.TODO(), require, s.tc, append(s.commonSources, s.smbshareSources...))
	s.waitForPods(s.serverLabelPattern)
}

func (s *MountPathPermissionsSuite) TearDownSuite() {
	deleteFromFiles(context.TODO(), s.Require(), s.tc, s.smbshareSources)
	deleteFromFiles(context.TODO(), s.Require(), s.tc, s.commonSources)
}

// The test checks the first directory within /mnt for the xattr.
// We may need to change this if multiple mount paths are supported
func (s *MountPathPermissionsSuite) TestMountPathPermissions() {
	require := s.Require()
	cmd := []string{
		"/bin/python3",
		"-c",
		"import xattr,os;mp='/mnt/' + os.listdir('/mnt')[0]; print(xattr.get(mp, 'user.share-perms-status'))",
	}

	pods, err := s.tc.FetchPods(
		context.TODO(),
		kube.PodFetchOptions{
			Namespace:     s.smbShareResource.Namespace,
			LabelSelector: s.serverLabelPattern,
			MaxFound:      1,
		})
	require.NoError(err)

	pc := kube.PodCommand{
		Command:       cmd,
		Namespace:     s.smbShareResource.Namespace,
		PodName:       pods[0].Name,
		ContainerName: "samba",
	}
	bch := kube.NewBufferedCommandHandler()
	err = kube.NewTestExec(s.tc).Call(pc, bch)
	require.NoError(err)
	out := strings.TrimSpace(string(bch.GetStdout()))
	s.Require().NotEmpty(out)
}
