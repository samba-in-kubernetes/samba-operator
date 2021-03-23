// +build integration

package integration

import (
	"context"
	"path"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

type ShareAccessSuite struct {
	suite.Suite

	share     smbclient.Share
	auths     []smbclient.Auth
	clientPod string
}

func (s *ShareAccessSuite) SetupSuite() {
	s.clientPod = "smbclient"

	// ensure the smbclient test pod is configured
	tc := kube.NewTestClient("")
	_, err := tc.CreateFromFileIfMissing(
		context.TODO(),
		kube.FileSource{
			Path:      path.Join(testFilesDir, "data1.yaml"),
			Namespace: testNamespace,
		})
	s.Require().NoError(err)
	_, err = tc.CreateFromFileIfMissing(
		context.TODO(),
		kube.FileSource{
			Path:      path.Join(testFilesDir, "client-test-pod.yaml"),
			Namespace: testNamespace,
		})
	s.Require().NoError(err)

	// ensure the smbclient test pod exists and is ready
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(120*time.Second))
	defer cancel()
	s.Require().NoError(kube.WaitForPodExistsByLabel(
		ctx,
		kube.NewTestClient(""),
		"app=samba-operator-test-smbclient",
		testNamespace),
		"smbclient pod does not exist",
	)
	s.Require().NoError(kube.WaitForPodReadyByLabel(
		ctx,
		kube.NewTestClient(""),
		"app=samba-operator-test-smbclient",
		testNamespace),
		"smbclient pod not ready",
	)
}

// TestLogin verifies that users can log into the share.
func (s *ShareAccessSuite) TestLogin() {
	smbclient := smbclient.MustPodClient(testNamespace, s.clientPod)
	for _, auth := range s.auths {
		err := smbclient.Command(
			context.TODO(),
			s.share,
			auth,
			[]string{"ls"})
		s.Require().NoError(err)
	}
}

func (s *ShareAccessSuite) TestPutFile() {
	smbclient := smbclient.MustPodClient(testNamespace, s.clientPod)
	auth := s.auths[0]
	err := smbclient.Command(
		context.TODO(),
		s.share,
		auth,
		[]string{"put profile.jpeg"})
	s.Require().NoError(err)
	out, err := smbclient.CommandOutput(
		context.TODO(),
		s.share,
		auth,
		[]string{"ls"})
	s.Require().NoError(err)
	s.Require().Contains(string(out), "profile.jpeg")
}
