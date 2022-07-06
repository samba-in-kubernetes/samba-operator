// +build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/types"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
)

type MountPathPermissionsSuite struct {
	suite.Suite

	commonSources   []kube.FileSource
	smbShareSources []kube.FileSource

	// tc is a TestClient instance
	tc *kube.TestClient

	// testID is a short unique test id, pseudo-randomly generated
	testID string
	// testShareName is the name of the SmbShare being tested by this
	// test instance
	testShareName types.NamespacedName
}

func (s *MountPathPermissionsSuite) defaultContext() context.Context {
	return testContext()
}

func (s *MountPathPermissionsSuite) waitForPods(labelPattern string) {
	require := s.Require()
	ctx, cancel := context.WithDeadline(
		s.defaultContext(),
		time.Now().Add(waitForPodsTime))
	defer cancel()
	opts := kube.PodFetchOptions{
		Namespace:     testNamespace,
		LabelSelector: labelPattern,
	}
	require.NoError(
		kube.WaitForAnyPodExists(ctx, s.tc, opts),
		"pod does not exist",
	)
	require.NoError(
		kube.WaitForAnyPodReady(ctx, s.tc, opts),
		"pod not ready",
	)
}

func (s *MountPathPermissionsSuite) SetupSuite() {
	s.testID = generateTestID()
	s.T().Logf("test ID: %s", s.testID)
	s.tc = kube.NewTestClient("")
	require := s.Require()

	// Create smbshare with Spec.Storage.PVC.Path specified
	ctx := s.defaultContext()
	createFromFiles(ctx, require, s.tc, s.commonSources)
	names := createFromFilesWithSuffix(
		ctx,
		s.Require(),
		s.tc,
		s.smbShareSources,
		s.testID,
	)
	s.Require().Len(names, 1, "expected one smb share resource")
	s.testShareName = names[0]
	lbl := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.testShareName.Name)
	s.waitForPods(lbl)
}

func (s *MountPathPermissionsSuite) TearDownSuite() {
	ctx := s.defaultContext()
	deleteFromFilesWithSuffix(ctx, s.Require(), s.tc, s.smbShareSources, s.testID)
	deleteFromFiles(ctx, s.Require(), s.tc, s.commonSources)
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

	lbl := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.testShareName.Name)
	pods, err := s.tc.FetchPods(
		s.defaultContext(),
		kube.PodFetchOptions{
			Namespace:     s.testShareName.Namespace,
			LabelSelector: lbl,
			MaxFound:      1,
		})
	require.NoError(err)

	pc := kube.PodCommand{
		Command:       cmd,
		Namespace:     s.testShareName.Namespace,
		PodName:       pods[0].Name,
		ContainerName: "samba",
	}
	bch := kube.NewBufferedCommandHandler()
	err = kube.NewTestExec(s.tc).Call(pc, bch)
	require.NoError(err)
	out := strings.TrimSpace(string(bch.GetStdout()))
	s.Require().NotEmpty(out)
}
