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

	// parentContext is a context provided by this test's parent.
	// some would consider this not a best practice, but it is
	// a simple way to know what smbshare/etc is under test.
	// We always assume this suite is executed by a higher level test
	parentContext context.Context
}

func (s *ShareAccessSuite) defaultContext() context.Context {
	if s.parentContext == nil {
		// fallback in case parentContext is unset
		return testContext()
	}
	return s.parentContext
}

func (s *ShareAccessSuite) SetupSuite() {
	s.clientPod = "smbclient"

	// ensure the smbclient test pod is configured
	createSMBClientIfMissing(s.defaultContext(), s.Require(), kube.NewTestClient(""))
}

// TestLogin verifies that users can log into the share.
func (s *ShareAccessSuite) TestLogin() {
	ctx := s.defaultContext()
	tc := kube.NewTestClient("")
	smbclient := smbclient.MustPodExec(tc, testNamespace, s.clientPod, "")
	err := smbclient.CacheFlush(ctx)
	s.Require().NoError(err)
	for _, auth := range s.auths {
		err := smbclient.Command(
			ctx,
			s.share,
			auth,
			[]string{"ls"})
		s.Require().NoError(err)
	}
}

func (s *ShareAccessSuite) TestPutFile() {
	ctx := s.defaultContext()
	tc := kube.NewTestClient("")
	smbclient := smbclient.MustPodExec(tc, testNamespace, s.clientPod, "")
	err := smbclient.CacheFlush(ctx)
	s.Require().NoError(err)
	auth := s.auths[0]
	err = smbclient.Command(
		ctx,
		s.share,
		auth,
		[]string{"put profile.jpeg"})
	s.Require().NoError(err)
	out, err := smbclient.CommandOutput(
		ctx,
		s.share,
		auth,
		[]string{"ls"})
	s.Require().NoError(err)
	s.Require().Contains(string(out), "profile.jpeg")
}
