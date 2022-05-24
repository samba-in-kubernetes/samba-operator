//go:build integration
// +build integration

package integration

import (
	"context"

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
	createSMBClientIfMissing(s.Require(), kube.NewTestClient(""))
}

// TestLogin verifies that users can log into the share.
func (s *ShareAccessSuite) TestLogin() {
	tc := kube.NewTestClient("")
	smbclient := smbclient.MustPodExec(tc, testNamespace, s.clientPod, "")
	err := smbclient.CacheFlush(context.TODO())
	s.Require().NoError(err)
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
	tc := kube.NewTestClient("")
	smbclient := smbclient.MustPodExec(tc, testNamespace, s.clientPod, "")
	err := smbclient.CacheFlush(context.TODO())
	s.Require().NoError(err)
	auth := s.auths[0]
	err = smbclient.Command(
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
