//go:build integration
// +build integration

// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"fmt"
	"math"
	"path"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"

	"k8s.io/apimachinery/pkg/types"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/poll"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

var (
	// waitForSyncTime is 1.5 times the default Kubelet syncFrequency
	waitForSyncTime = 90 * time.Second
)

type SmbShareUpdateSuite struct {
	suite.Suite

	commonSources   []kube.FileSource
	smbShareSources []kube.FileSource
	shareName       string
	testAuths       []smbclient.Auth
	destNamespace   string

	// cached values
	tc *kube.TestClient

	// testID is a short unique test id, pseudo-randomly generated
	testID string
	// testShareName is the name of the SmbShare being tested by this
	// test instance
	testShareName types.NamespacedName
}

func (s *SmbShareUpdateSuite) defaultContext() context.Context {
	ctx := testContext()
	if s.testID != "" {
		ctx = context.WithValue(ctx, TestIDKey, s.testID)
	}
	if s.testShareName.Name != "" {
		ctx = context.WithValue(ctx, TestShareKey,
			fmt.Sprintf("%s/%s",
				s.testShareName.Namespace,
				s.testShareName.Name))
	}
	return ctx
}

func (s *SmbShareUpdateSuite) getTestClient() *kube.TestClient {
	return s.tc
}

func (s *SmbShareUpdateSuite) getPodFetchOptions() kube.PodFetchOptions {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.testShareName.Name)
	return kube.PodFetchOptions{
		Namespace:     s.destNamespace,
		LabelSelector: l,
	}
}

func (s *SmbShareUpdateSuite) SetupSuite() {
	s.testID = generateTestID()
	s.T().Logf("test ID: %s", s.testID)
	s.Require().NotEmpty(s.destNamespace)
	s.Require().Len(
		s.smbShareSources, 1, "currently only one share may be tested")
	s.tc = kube.NewTestClient("")
	// ensure the smbclient test pod exists
	ctx := s.defaultContext()
	createSMBClientIfMissing(ctx, s.Require(), s.tc)
	createFromFiles(ctx, s.Require(), s.tc, s.commonSources)
	names := createFromFilesWithSuffix(
		ctx,
		s.Require(),
		s.tc,
		s.smbShareSources,
		s.testID,
	)
	s.Require().Len(names, 1, "expected one smb share resource")
	s.testShareName = names[0]
}

func (s *SmbShareUpdateSuite) SetupTest() {
	ctx := s.defaultContext()
	require := s.Require()
	require.NoError(waitForPodExist(ctx, s), "smb server pod does not exist")
	require.NoError(waitForPodReady(ctx, s), "smb server pod is not ready")
}

func (s *SmbShareUpdateSuite) TearDownSuite() {
	ctx := s.defaultContext()
	deleteFromFiles(ctx, s.Require(), s.tc, s.commonSources)
	deleteFromFilesWithSuffix(
		ctx,
		s.Require(),
		s.tc,
		s.smbShareSources,
		s.testID)
	s.waitForCleanup()
}

func (s *SmbShareUpdateSuite) waitForCleanup() {
	ctx, cancel := context.WithTimeout(
		s.defaultContext(),
		waitForCleanupTime)
	defer cancel()
	err := poll.TryUntil(ctx, &poll.Prober{
		RetryInterval: time.Second,
		Cond: func() (bool, error) {
			// set max pods since were waiting for "drain"
			fopts := s.getPodFetchOptions()
			fopts.MaxFound = math.MaxInt32
			_, err := s.tc.FetchPods(ctx, fopts)
			if err == kube.ErrNoMatchingPods {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			s.T().Logf("pod matching [%s] in [%s] still exists",
				fopts.LabelSelector,
				fopts.Namespace)
			return false, nil
		},
	})
	s.Require().NoError(err)
}

func (s *SmbShareUpdateSuite) TestEditReadOnly() {
	ctx := s.defaultContext()
	require := s.Require()

	// first make sure we can even log into the share at all
	s.T().Log("checking smbclient login to share")
	client := smbclient.MustPodExec(s.tc, testNamespace,
		"smbclient", "client")
	require.NoError(client.CacheFlush(ctx))
	requireSMBLogin(ctx, require, client, s.destShare(), s.testAuths)

	// first we set the share to read only.
	// this lets us hit the "error" first before setting it
	// back original value for the share
	s.setReadOnly(ctx, true)
	// wait a bit for cm changes to appear in pod
	time.Sleep(20 * time.Second)

	// test that the share has become read only
	s.putFile(ctx, func(e error) bool {
		return e != nil && strings.Contains(
			e.Error(), "NT_STATUS_ACCESS_DENIED")
	})

	// restore the share to read-write
	s.setReadOnly(ctx, false)
	// wait a bit for cm changes to appear in pod
	time.Sleep(20 * time.Second)

	// test that the share is writable again
	s.putFile(ctx, func(e error) bool {
		return e == nil
	})
}

func (s *SmbShareUpdateSuite) setReadOnly(ctx context.Context, ro bool) {
	require := s.Require()
	smbShare := &sambaoperatorv1alpha1.SmbShare{}
	err := s.tc.TypedObjectClient().Get(
		ctx, s.testShareName, smbShare)
	require.NoError(err)

	s.T().Logf("Setting readonly=%v for SmbShare %s/%s",
		ro,
		smbShare.Namespace,
		smbShare.Name)
	smbShare.Spec.ReadOnly = ro
	err = s.tc.TypedObjectClient().Update(
		ctx, smbShare)
	require.NoError(err)
}

func (s *SmbShareUpdateSuite) destShare() smbclient.Share {
	svcname := fmt.Sprintf("%s.%s.svc.cluster.local",
		s.testShareName.Name,
		s.testShareName.Namespace)
	share := smbclient.Share{
		Host: smbclient.Host(svcname),
		Name: s.shareName,
	}
	return share
}

func (s *SmbShareUpdateSuite) putFile(
	ctx context.Context,
	check func(error) bool) error {
	// ---
	var err error
	ctx2, cancel := context.WithTimeout(
		s.defaultContext(),
		waitForSyncTime)
	defer cancel()

	auth := s.testAuths[0]
	poll.TryUntil(ctx2, &poll.Prober{
		RetryInterval: 10 * time.Second,
		Cond: func() (bool, error) {
			client := smbclient.MustPodExec(s.tc, testNamespace,
				"smbclient", "client")
			err = client.CacheFlush(ctx2)
			if err != nil {
				return false, err
			}
			s.T().Log("checking smbclient write to share")
			err = client.Command(
				ctx2,
				s.destShare(),
				auth,
				[]string{"put profile.jpeg"})
			return check(err), nil
		},
	})
	return err
}

func init() {
	resourceUpdateTests := testRoot.ChildPriority("resourceUpdate", 5)
	resourceUpdateTests.AddSuite("SmbShareUpdateSuite", &SmbShareUpdateSuite{
		commonSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: testNamespace,
			},
		},
		smbShareSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "smbshare1.yaml"),
				Namespace: testNamespace,
			},
		},
		destNamespace: testNamespace,
		shareName:     "My Share",
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
	})
}
