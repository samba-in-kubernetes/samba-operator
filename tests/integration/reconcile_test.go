//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/types"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/poll"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	backendAnnotation = "samba-operator.samba.org/serverBackend"
)

type limitAvailModeChangeSuite struct {
	suite.Suite

	commonSources   []kube.FileSource
	smbShareSources []kube.FileSource
	nextMode        string
	expectBackend   string

	// cached values
	tc *kube.TestClient

	// testID is a short unique test id, pseudo-randomly generated
	testID string
	// testShareName is the name of the SmbShare being tested by this
	// test instance
	testShareName types.NamespacedName
}

func (s *limitAvailModeChangeSuite) defaultContext() context.Context {
	return testContext()
}

func (s *limitAvailModeChangeSuite) SetupSuite() {
	s.testID = generateTestID()
	s.T().Logf("test ID: %s", s.testID)
	// ensure the smbclient test pod exists
	require := s.Require()
	s.tc = kube.NewTestClient("")
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
	require.NoError(waitForPodExist(ctx, s), "smb server pod does not exist")
	require.NoError(waitForPodReady(ctx, s), "smb server pod is not ready")
}

func (s *limitAvailModeChangeSuite) TearDownSuite() {
	ctx := s.defaultContext()
	deleteFromFiles(ctx, s.Require(), s.tc, s.commonSources)
	deleteFromFilesWithSuffix(
		ctx,
		s.Require(),
		s.tc,
		s.smbShareSources,
		s.testID)
}

func (s *limitAvailModeChangeSuite) getTestClient() *kube.TestClient {
	return s.tc
}

func (s *limitAvailModeChangeSuite) getPodFetchOptions() kube.PodFetchOptions {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.testShareName.Name)
	return kube.PodFetchOptions{
		Namespace:     s.testShareName.Namespace,
		LabelSelector: l,
		MaxFound:      3,
	}
}

func (s *limitAvailModeChangeSuite) TestAvailModeUnchanged() {
	ctx := s.defaultContext()
	require := s.Require()
	smbShare := &sambaoperatorv1alpha1.SmbShare{}
	err := s.tc.TypedObjectClient().Get(
		ctx, s.testShareName, smbShare)
	require.NoError(err)
	require.NotNil(smbShare.Annotations)
	require.Contains(smbShare.Annotations[backendAnnotation], s.expectBackend)
	if smbShare.Spec.Scaling == nil {
		smbShare.Spec.Scaling = &sambaoperatorv1alpha1.SmbShareScalingSpec{}
	}
	smbShare.Spec.Scaling.AvailabilityMode = s.nextMode
	smbShare.Spec.Scaling.MinClusterSize = 2

	err = s.tc.TypedObjectClient().Update(
		ctx, smbShare)
	require.NoError(err)
	require.NoError(waitForPodExist(ctx, s), "smb server pod does not exist")
	require.NoError(waitForPodReady(ctx, s), "smb server pod is not ready")
	err = s.tc.TypedObjectClient().Get(
		ctx, s.testShareName, smbShare)
	require.NoError(err)
	require.NotNil(smbShare.Annotations)
	require.Contains(smbShare.Annotations[backendAnnotation], s.expectBackend)
}

type scaleoutClusterSuite struct {
	suite.Suite

	commonSources   []kube.FileSource
	smbShareSources []kube.FileSource
	maxPods         int
	minPods         int

	// cached values
	tc *kube.TestClient

	// testID is a short unique test id, pseudo-randomly generated
	testID string
	// testShareName is the name of the SmbShare being tested by this
	// test instance
	testShareName types.NamespacedName
}

func (s *scaleoutClusterSuite) defaultContext() context.Context {
	return testContext()
}

func (s *scaleoutClusterSuite) SetupSuite() {
	s.testID = generateTestID()
	s.T().Logf("test ID: %s", s.testID)
	// ensure the smbclient test pod exists
	ctx := s.defaultContext()
	require := s.Require()
	s.tc = kube.NewTestClient("")
	createSMBClientIfMissing(ctx, require, s.tc)
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
	require.NoError(waitForPodExist(ctx, s), "smb server pod does not exist")
	require.NoError(waitForPodReady(ctx, s), "smb server pod is not ready")
}

func (s *scaleoutClusterSuite) TearDownSuite() {
	ctx := s.defaultContext()
	deleteFromFiles(ctx, s.Require(), s.tc, s.commonSources)
	deleteFromFilesWithSuffix(
		ctx,
		s.Require(),
		s.tc,
		s.smbShareSources,
		s.testID)
}

func (s *scaleoutClusterSuite) getTestClient() *kube.TestClient {
	return s.tc
}

func (s *scaleoutClusterSuite) getPodFetchOptions() kube.PodFetchOptions {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.testShareName.Name)
	return kube.PodFetchOptions{
		Namespace:     s.testShareName.Namespace,
		LabelSelector: l,
		MaxFound:      s.maxPods,
		MinFound:      s.minPods,
	}
}

func (s *scaleoutClusterSuite) TestScaleoutClusterSuite() {
	ctx := s.defaultContext()
	require := s.Require()
	smbShare := &sambaoperatorv1alpha1.SmbShare{}
	err := s.tc.TypedObjectClient().Get(
		ctx, s.testShareName, smbShare)
	require.NoError(err)

	// Increase Cluster Size by 1 and check result
	newClusterSize := smbShare.Spec.Scaling.MinClusterSize + 1
	smbShare.Spec.Scaling.MinClusterSize = newClusterSize
	err = s.tc.TypedObjectClient().Update(
		ctx, smbShare)
	require.NoError(err)

	ctx2, cancel := context.WithTimeout(s.defaultContext(), 3*time.Second)
	defer cancel()
	s.Require().NoError(poll.TryUntil(ctx2, &poll.Prober{
		RetryInterval: time.Second,
		Cond: func() (bool, error) {
			return s.checkStatefulSet(ctx, int32(newClusterSize)), nil
		},
	}))

	s.minPods = newClusterSize
	require.NoError(waitForPodExist(ctx, s), "smb server pod does not exist")
}

func (s *scaleoutClusterSuite) checkStatefulSet(
	ctx context.Context, cSize int32) bool {
	l, err := s.tc.Clientset().AppsV1().StatefulSets(s.testShareName.Namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("samba-operator.samba.org/service=%s",
				s.testShareName.Name),
		})
	if err != nil {
		s.T().Logf("StatefulSet couldn't be listed")
		return false
	}

	if len(l.Items) != 1 {
		// Only one stateful set should be available for this smbshare.
		s.T().Logf("StatefulSet count does not match")
		return false
	}
	if *l.Items[0].Spec.Replicas != cSize {
		// Replicas field should reflect updated MinClusterSize
		s.T().Logf("StatefulSet Replicas not yet updated")
		return false
	}
	return true
}

func init() {
	if !testClusteredShares {
		return
	}

	reconTests := testRoot.ChildPriority("reconciliation", 4)
	reconTests.AddSuite("limitAvailModeChangeStandard", &limitAvailModeChangeSuite{
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
		expectBackend: "standard",
		nextMode:      "clustered",
	},
	)

	reconTests.AddSuite("limitAvailModeChangeClustered", &limitAvailModeChangeSuite{
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
				Path:      path.Join(testFilesDir, "smbshare_ctdb1.yaml"),
				Namespace: testNamespace,
			},
		},
		expectBackend: "clustered",
		nextMode:      "standard",
	},
	)

	reconTests.AddSuite("scaleoutCluster", &scaleoutClusterSuite{
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
				Path:      path.Join(testFilesDir, "smbshare_ctdb1.yaml"),
				Namespace: testNamespace,
			},
		},
		maxPods: 3,
		minPods: 2,
	},
	)
}
