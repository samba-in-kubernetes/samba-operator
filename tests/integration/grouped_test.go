//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"math"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/poll"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

type GroupedSharesSuite struct {
	suite.Suite

	commonSources      []kube.FileSource
	smbShareSources    []kube.FileSource
	toDelete           []types.NamespacedName
	phaseOneShareNames []string
	phaseTwoShareNames []string
	goneShareName      string
	testAuths          []smbclient.Auth
	destNamespace      string
	maxPods            int
	minPods            int

	// cached values
	tc *kube.TestClient

	// testID is a short unique test id, pseudo-randomly generated
	testID string
	// share resources created
	testShareNames []types.NamespacedName
	// server group name assigned by the operator
	serverGroupName string
}

func (s *GroupedSharesSuite) defaultContext() context.Context {
	ctx := testContext()
	if s.testID != "" {
		ctx = context.WithValue(ctx, TestIDKey, s.testID)
	}
	return ctx
}

func (s *GroupedSharesSuite) SetupSuite() {
	s.testID = generateTestID()
	s.T().Logf("test ID: %s", s.testID)
	s.tc = kube.NewTestClient("")
	// ensure the smbclient test pod exists
	ctx := s.defaultContext()
	createSMBClientIfMissing(ctx, s.Require(), s.tc)
	createFromFiles(ctx, s.Require(), s.tc, s.commonSources)
	s.testShareNames = createFromFilesWithSuffix(
		ctx,
		s.Require(),
		s.tc,
		s.smbShareSources,
		s.testID,
	)
}

func (s *GroupedSharesSuite) SetupTest() {
	require := s.Require()
	ctx, cancel := context.WithTimeout(
		s.defaultContext(),
		waitForReadyTime*2)
	defer cancel()
	s.serverGroupName = s.getServerGroupName(ctx)
	require.NoError(waitForPodExist(ctx, s), "smb server pod does not exist")
	require.NoError(waitForPodReady(ctx, s), "smb server pod is not ready")
}

func (s *GroupedSharesSuite) TearDownSuite() {
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

func (s *GroupedSharesSuite) waitForCleanup() {
	ctx, cancel := context.WithTimeout(
		s.defaultContext(),
		waitForCleanupTime)
	defer cancel()
	s.Require().NotEqual(s.serverGroupName, "")
	err := poll.TryUntil(ctx, &poll.Prober{
		RetryInterval: time.Second,
		Cond: func() (bool, error) {
			lbl := fmt.Sprintf(
				"samba-operator.samba.org/service=%s",
				s.serverGroupName)
			// only set max pods since were waiting for "drain"
			_, err := s.tc.FetchPods(
				ctx,
				kube.PodFetchOptions{
					Namespace:     s.destNamespace,
					LabelSelector: lbl,
					MaxFound:      math.MaxInt32,
				})
			if err == kube.ErrNoMatchingPods {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			s.T().Logf("pod with label [%s] still exists", lbl)
			return false, nil
		},
	})
	s.Require().NoError(err)
}

func (s *GroupedSharesSuite) getTestClient() *kube.TestClient {
	return s.tc
}

func (s *GroupedSharesSuite) getServerGroupName(ctx context.Context) string {
	smbShare := &sambaoperatorv1alpha1.SmbShare{}
	for {
		for _, nn := range s.testShareNames {
			err := s.tc.TypedObjectClient().Get(ctx, nn, smbShare)
			if err != nil {
				s.T().Logf("failed to get smbShare: %s", err.Error())
				continue
			}
			s.T().Logf("serverGroup on %s/%s = '%s'",
				nn.Namespace,
				nn.Name,
				smbShare.Status.ServerGroup)
			if smbShare.Status.ServerGroup != "" {
				return smbShare.Status.ServerGroup
			}
		}
	}
	return ""
}

func (s *GroupedSharesSuite) getPodFetchOptions() kube.PodFetchOptions {
	s.Require().NotEqual(s.serverGroupName, "")
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.serverGroupName)
	return kube.PodFetchOptions{
		Namespace:     s.destNamespace,
		LabelSelector: l,
		MaxFound:      s.maxPods,
		MinFound:      s.minPods,
	}
}

func (s *GroupedSharesSuite) TestPodsReady() {
	s.Require().NoError(waitForPodReady(s.defaultContext(), s))
}

func (s *GroupedSharesSuite) waitForShareGone(
	ctx context.Context) error {
	// ---
	var err error
	ctx2, cancel := context.WithTimeout(
		s.defaultContext(),
		waitForSyncTime)
	defer cancel()

	svcname := fmt.Sprintf("%s.%s.svc.cluster.local",
		s.serverGroupName,
		s.destNamespace)
	share := smbclient.Share{
		Host: smbclient.Host(svcname),
		Name: s.goneShareName,
	}
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
			s.T().Logf("checking smbclient can write to share %s", share)
			err = client.Command(
				ctx2,
				share,
				auth,
				[]string{"put profile.jpeg"})
			if err != nil && strings.Contains(err.Error(), "BAD_NETWORK_NAME") {
				return true, nil
			}
			if err != nil {
				s.T().Logf("got unexpected error: %s", err.Error())
			}
			return false, nil
		},
	})
	return err
}

func (s *GroupedSharesSuite) TestPhases() {
	phaseTest := func(t *testing.T, shareNames []string) {
		t.Run("byServiceName", func(t *testing.T) {
			for _, shareName := range shareNames {
				t.Run(shareName, func(t *testing.T) {
					testShareAccessByServiceName(t, s, shareName)
				})
			}
		})
		t.Run("byIP", func(t *testing.T) {
			for _, shareName := range shareNames {
				t.Run(shareName, func(t *testing.T) {
					testShareAccessByIP(t, s, shareName)
				})
			}
		})
	}

	s.T().Run("phaseOne", func(t *testing.T) {
		phaseTest(t, s.phaseOneShareNames)
	})

	s.T().Log("about to delete")
	time.Sleep(3 * time.Second)
	s.T().Log("deleting!")

	for _, nn := range s.toDelete {
		smbShare := &sambaoperatorv1alpha1.SmbShare{}
		for _, tsn := range s.testShareNames {
			if strings.HasPrefix(tsn.Name, nn.Name) {
				smbShare.Namespace = tsn.Namespace
				smbShare.Name = tsn.Name
			}
		}
		s.Require().NotEmpty(smbShare.Name)
		err := s.tc.TypedObjectClient().Delete(s.defaultContext(), smbShare)
		s.Require().NoError(err)
	}

	s.waitForShareGone(s.defaultContext())

	s.T().Run("phaseTwo", func(t *testing.T) {
		phaseTest(t, s.phaseTwoShareNames)
	})
}

func testShareAccessByServiceName(
	t *testing.T, s *GroupedSharesSuite, shareName string) {
	// ---
	svcname := fmt.Sprintf("%s.%s.svc.cluster.local",
		s.serverGroupName,
		s.destNamespace)
	shareAccessSuite := &ShareAccessSuite{
		share: smbclient.Share{
			Host: smbclient.Host(svcname),
			Name: shareName,
		},
		auths: s.testAuths,
		// pass a context for tracking
		parentContext: s.defaultContext(),
	}
	suite.Run(t, shareAccessSuite)
}

func testShareAccessByIP(
	t *testing.T, s *GroupedSharesSuite, shareName string) {
	// ---
	ip, err := getAnyPodIP(s.defaultContext(), s)
	s.Require().NoError(err)
	shareAccessSuite := &ShareAccessSuite{
		share: smbclient.Share{
			Host: smbclient.Host(ip),
			Name: shareName,
		},
		auths: s.testAuths,
		// pass a context for tracking
		parentContext: s.defaultContext(),
	}
	suite.Run(t, shareAccessSuite)
}

type InvalidGroupingsSuite struct {
	suite.Suite

	commonSources []kube.FileSource
	destNamespace string

	// cached values
	tc *kube.TestClient

	// testID is a short unique test id, pseudo-randomly generated
	testID string
}

func (s *InvalidGroupingsSuite) defaultContext() context.Context {
	ctx := testContext()
	if s.testID != "" {
		ctx = context.WithValue(ctx, TestIDKey, s.testID)
	}
	return ctx
}

func (s *InvalidGroupingsSuite) SetupSuite() {
	s.testID = generateTestID()
	s.T().Logf("test ID: %s", s.testID)
	s.tc = kube.NewTestClient("")
	// ensure the smbclient test pod exists
	ctx := s.defaultContext()
	createSMBClientIfMissing(ctx, s.Require(), s.tc)
	createFromFiles(ctx, s.Require(), s.tc, s.commonSources)
}

func (s *InvalidGroupingsSuite) TearDownSuite() {
	ctx := s.defaultContext()
	deleteFromFiles(ctx, s.Require(), s.tc, s.commonSources)
}

func (s *InvalidGroupingsSuite) TestGroupNeverWithName() {
	smbShare := &sambaoperatorv1alpha1.SmbShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "badmode-" + s.testID,
			Namespace: s.destNamespace,
		},
		Spec: sambaoperatorv1alpha1.SmbShareSpec{
			ShareName:      "badmode",
			ReadOnly:       false,
			Browseable:     true,
			SecurityConfig: "sharesec1",
			Scaling: &sambaoperatorv1alpha1.SmbShareScalingSpec{
				GroupMode: "never",
				Group:     "noway",
			},
			Storage: sambaoperatorv1alpha1.SmbShareStorageSpec{
				Pvc: &sambaoperatorv1alpha1.SmbSharePvcSpec{
					Name: "badmode",
					Path: "badmode",
				},
			},
		},
	}
	err := s.tc.TypedObjectClient().Create(
		s.defaultContext(), smbShare)
	s.Require().NoError(err)
	defer func() {
		err := s.tc.TypedObjectClient().Delete(
			s.defaultContext(), smbShare)
		s.Require().NoError(err)
	}()

	// check that there was an error from the operator
	ctx, cancel := context.WithTimeout(
		s.defaultContext(),
		time.Second*60)
	defer cancel()
	sel := fmt.Sprintf("involvedObject.kind=SmbShare,involvedObject.name=%s,involvedObject.uid=%s", smbShare.Name, smbShare.GetUID())
	err = poll.TryUntil(ctx, &poll.Prober{
		RetryInterval: time.Second,
		Cond: func() (bool, error) {
			l, err := s.tc.Clientset().CoreV1().Events(smbShare.Namespace).List(
				s.defaultContext(),
				metav1.ListOptions{
					FieldSelector: sel,
				})
			if err != nil {
				return false, err
			}
			found := 0
			s.T().Logf("found %d events", len(l.Items))
			for _, event := range l.Items {
				s.T().Logf("event: %s %s", event.Reason, event.Message)
				if event.Reason == "InvalidConfiguration" {
					found++
				}
			}
			return found > 0, nil
		},
	})
	s.Require().NoError(err, "didn't find expected events")
}

func (s *InvalidGroupingsSuite) TestGroupExplicitNoName() {
	smbShare := &sambaoperatorv1alpha1.SmbShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "badmode2-" + s.testID,
			Namespace: s.destNamespace,
		},
		Spec: sambaoperatorv1alpha1.SmbShareSpec{
			ShareName:      "badmode2",
			ReadOnly:       false,
			Browseable:     true,
			SecurityConfig: "sharesec1",
			Scaling: &sambaoperatorv1alpha1.SmbShareScalingSpec{
				GroupMode: "explicit",
				Group:     "",
			},
			Storage: sambaoperatorv1alpha1.SmbShareStorageSpec{
				Pvc: &sambaoperatorv1alpha1.SmbSharePvcSpec{
					Name: "badmode2",
					Path: "badmode",
				},
			},
		},
	}
	err := s.tc.TypedObjectClient().Create(
		s.defaultContext(), smbShare)
	s.Require().NoError(err)
	defer func() {
		err := s.tc.TypedObjectClient().Delete(
			s.defaultContext(), smbShare)
		s.Require().NoError(err)
	}()

	// check that there was an error from the operator
	ctx, cancel := context.WithTimeout(
		s.defaultContext(),
		time.Second*60)
	defer cancel()
	sel := fmt.Sprintf("involvedObject.kind=SmbShare,involvedObject.name=%s,involvedObject.uid=%s", smbShare.Name, smbShare.GetUID())
	err = poll.TryUntil(ctx, &poll.Prober{
		RetryInterval: time.Second,
		Cond: func() (bool, error) {
			l, err := s.tc.Clientset().CoreV1().Events(smbShare.Namespace).List(
				s.defaultContext(),
				metav1.ListOptions{
					FieldSelector: sel,
				})
			if err != nil {
				return false, err
			}
			found := 0
			s.T().Logf("found %d events", len(l.Items))
			for _, event := range l.Items {
				s.T().Logf("event: %s %s", event.Reason, event.Message)
				if event.Reason == "InvalidConfiguration" {
					found++
				}
			}
			return found > 0, nil
		},
	})
	s.Require().NoError(err, "didn't find expected events")
}

func (s *InvalidGroupingsSuite) TestIncompatible() {
	smbShare1 := &sambaoperatorv1alpha1.SmbShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "badmerge1-" + s.testID,
			Namespace: s.destNamespace,
		},
		Spec: sambaoperatorv1alpha1.SmbShareSpec{
			ShareName:      "badmerge1",
			ReadOnly:       false,
			Browseable:     true,
			SecurityConfig: "sharesec1",
			Scaling: &sambaoperatorv1alpha1.SmbShareScalingSpec{
				GroupMode: "explicit",
				Group:     "grouppls",
			},
			Storage: sambaoperatorv1alpha1.SmbShareStorageSpec{
				Pvc: &sambaoperatorv1alpha1.SmbSharePvcSpec{
					Name: "badmerge",
					Path: "badmerge1",
				},
			},
		},
	}
	err := s.tc.TypedObjectClient().Create(
		s.defaultContext(), smbShare1)
	s.Require().NoError(err)
	defer func() {
		err := s.tc.TypedObjectClient().Delete(
			s.defaultContext(), smbShare1)
		s.Require().NoError(err)
	}()

	smbShare2 := &sambaoperatorv1alpha1.SmbShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "badmerge2-" + s.testID,
			Namespace: s.destNamespace,
		},
		Spec: sambaoperatorv1alpha1.SmbShareSpec{
			ShareName:      "badmerge2",
			ReadOnly:       false,
			Browseable:     true,
			SecurityConfig: "adsec1",
			Scaling: &sambaoperatorv1alpha1.SmbShareScalingSpec{
				GroupMode: "explicit",
				Group:     "grouppls",
			},
			Storage: sambaoperatorv1alpha1.SmbShareStorageSpec{
				Pvc: &sambaoperatorv1alpha1.SmbSharePvcSpec{
					Name: "badmerge",
					Path: "badmerge2",
				},
			},
		},
	}
	err = s.tc.TypedObjectClient().Create(
		s.defaultContext(), smbShare2)
	s.Require().NoError(err)
	defer func() {
		err := s.tc.TypedObjectClient().Delete(
			s.defaultContext(), smbShare2)
		s.Require().NoError(err)
	}()

	// check that there was an error from the operator
	ctx, cancel := context.WithTimeout(
		s.defaultContext(),
		time.Second*60)
	defer cancel()
	sel := fmt.Sprintf("involvedObject.kind=SmbShare,involvedObject.name=%s,involvedObject.uid=%s", smbShare2.Name, smbShare2.GetUID())
	err = poll.TryUntil(ctx, &poll.Prober{
		RetryInterval: time.Second,
		Cond: func() (bool, error) {
			l, err := s.tc.Clientset().CoreV1().Events(smbShare2.Namespace).List(
				s.defaultContext(),
				metav1.ListOptions{
					FieldSelector: sel,
				})
			if err != nil {
				return false, err
			}
			found := 0
			s.T().Logf("found %d events", len(l.Items))
			for _, event := range l.Items {
				s.T().Logf("event: %s %s", event.Reason, event.Message)
				if event.Reason == "InvalidConfiguration" {
					found++
				}
			}
			return found > 0, nil
		},
	})
	s.Require().NoError(err, "didn't find expected events")
}

func init() {
	groupedSharesTest := testRoot.ChildPriority("groupedShares", 6)
	groupedSharesTest.AddSuite("first", &GroupedSharesSuite{
		commonSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "cross-pvc1.yaml"),
				Namespace: testNamespace,
			},
		},
		smbShareSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "cross-share1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "cross-share2.yaml"),
				Namespace: testNamespace,
			},
		},
		toDelete: []types.NamespacedName{
			{Namespace: testNamespace, Name: "cross-share2"},
		},
		phaseOneShareNames: []string{
			"Cross One",
			"Cross Two",
		},
		phaseTwoShareNames: []string{
			"Cross One",
		},
		goneShareName: "Cross Two", // only specify one share that was removed
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
		destNamespace: testNamespace,
		maxPods:       1,
		minPods:       1,
	})
	groupedSharesTest.AddSuite("second", &GroupedSharesSuite{
		commonSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "cross-pvc1.yaml"),
				Namespace: testNamespace,
			},
		},
		smbShareSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "cross-share1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "cross-share2.yaml"),
				Namespace: testNamespace,
			},
		},
		toDelete: []types.NamespacedName{
			{Namespace: testNamespace, Name: "cross-share1"},
		},
		phaseOneShareNames: []string{
			"Cross One",
			"Cross Two",
		},
		phaseTwoShareNames: []string{
			"Cross Two",
		},
		goneShareName: "Cross One", // only specify one share that was removed
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
		destNamespace: testNamespace,
		maxPods:       1,
		minPods:       1,
	})
	groupedSharesTest.AddSuite("invalidGroupings", &InvalidGroupingsSuite{
		// this test will create share resources with differing security
		// configs. We want all the security related resources set up to
		// ensure missing resources can't possibly cause false negatives.
		commonSources: []kube.FileSource{
			// the resources for local security
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: testNamespace,
			},
			// the resources for ad security
			{
				Path:      path.Join(testFilesDir, "joinsecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig2.yaml"),
				Namespace: testNamespace,
			},
		},
		destNamespace: testNamespace,
	})
}
