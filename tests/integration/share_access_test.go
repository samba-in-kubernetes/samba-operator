//go:build integration
// +build integration

package integration

import (
	"context"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/poll"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

const (
	loginTestTimeout  = 10 * time.Second
	loginTestInterval = 500 * time.Millisecond
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
	require := s.Require()
	tc := kube.NewTestClient("")
	client := smbclient.MustPodExec(tc, testNamespace, s.clientPod, "")
	err := client.CacheFlush(ctx)
	require.NoError(err)

	ctx2, cancel := context.WithTimeout(ctx, loginTestTimeout)
	defer cancel()
	for _, auth := range s.auths {
		var cmderr error
		err := poll.TryUntil(ctx2, &poll.Prober{
			RetryInterval: loginTestInterval,
			Cond: func() (bool, error) {
				cmderr = client.Command(
					ctx,
					s.share,
					auth,
					[]string{"ls"})
				return cmderr == nil, nil
			},
		})
		// first check that cmderr is nil in order to capture the (much more
		// relevant) error that client.Command returned. if cmderr == nil
		// then err == nil. checking err at all is just a belt-and-suspenders
		// extra check in case something unexpected happens.
		require.NoError(cmderr)
		require.NoError(err)
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
