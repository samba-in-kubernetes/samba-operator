//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"path"

	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/types"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	backendAnnotation = "samba-operator.samba.org/serverBackend"
)

type limitAvailModeChangeSuite struct {
	suite.Suite

	fileSources     []kube.FileSource
	smbshareSources []kube.FileSource
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
	createFromFiles(ctx, require, s.tc, s.fileSources)
	names := createFromFilesWithSuffix(
		ctx,
		s.Require(),
		s.tc,
		s.smbshareSources,
		s.testID,
	)
	s.Require().Len(names, 1, "expected one smb share resource")
	s.testShareName = names[0]
	require.NoError(waitForPodExist(ctx, s), "smb server pod does not exist")
	require.NoError(waitForPodReady(ctx, s), "smb server pod is not ready")
}

func (s *limitAvailModeChangeSuite) TearDownSuite() {
	ctx := s.defaultContext()
	deleteFromFiles(ctx, s.Require(), s.tc, s.fileSources)
	deleteFromFilesWithSuffix(
		ctx,
		s.Require(),
		s.tc,
		s.smbshareSources,
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

	fileSources     []kube.FileSource
	smbshareSources []kube.FileSource

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
	createFromFiles(ctx, require, s.tc, s.fileSources)
	names := createFromFilesWithSuffix(
		ctx,
		s.Require(),
		s.tc,
		s.smbshareSources,
		s.testID,
	)
	s.Require().Len(names, 1, "expected one smb share resource")
	s.testShareName = names[0]
	require.NoError(waitForPodExist(ctx, s), "smb server pod does not exist")
	require.NoError(waitForPodReady(ctx, s), "smb server pod is not ready")
}

func (s *scaleoutClusterSuite) TearDownSuite() {
	ctx := s.defaultContext()
	deleteFromFiles(ctx, s.Require(), s.tc, s.fileSources)
	deleteFromFilesWithSuffix(
		ctx,
		s.Require(),
		s.tc,
		s.smbshareSources,
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
		MaxFound:      3,
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
	require.NoError(waitForPodExist(ctx, s), "smb server pod does not exist")
	require.NoError(waitForPodReady(ctx, s), "smb server pod is not ready")

	l, err := s.tc.Clientset().AppsV1().StatefulSets(s.testShareName.Namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("samba-operator.samba.org/service=%s",
				s.testShareName.Name),
		})
	require.NoError(err)
	// Only one stateful set should be available for this smbshare.
	require.Len(l.Items, 1)
	require.Equal(int32(newClusterSize), *l.Items[0].Spec.Replicas, "Clustersize not as expected")
}

func init() {
	if !testClusteredShares {
		return
	}

	reconTests := testRoot.ChildPriority("reconciliation", 4)
	reconTests.AddSuite("limitAvailModeChangeStandard", &limitAvailModeChangeSuite{
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: testNamespace,
			},
		},
		smbshareSources: []kube.FileSource{
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
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: testNamespace,
			},
		},
		smbshareSources: []kube.FileSource{
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
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: testNamespace,
			},
		},
		smbshareSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "smbshare_ctdb1.yaml"),
				Namespace: testNamespace,
			},
		},
	},
	)
}
