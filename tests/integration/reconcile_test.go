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

	fileSources      []kube.FileSource
	smbShareResource types.NamespacedName
	nextMode         string
	expectBackend    string

	// cached values
	tc *kube.TestClient
}

func (s *limitAvailModeChangeSuite) SetupSuite() {
	// ensure the smbclient test pod exists
	require := s.Require()
	s.tc = kube.NewTestClient("")
	createFromFiles(require, s.tc, s.fileSources)
	require.NoError(waitForPodExist(s), "smb server pod does not exist")
	require.NoError(waitForPodReady(s), "smb server pod is not ready")
}

func (s *limitAvailModeChangeSuite) TearDownSuite() {
	deleteFromFiles(s.Require(), s.tc, s.fileSources)
}

func (s *limitAvailModeChangeSuite) getTestClient() *kube.TestClient {
	return s.tc
}

func (s *limitAvailModeChangeSuite) getPodFetchOptions() kube.PodFetchOptions {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	return kube.PodFetchOptions{
		Namespace:     s.smbShareResource.Namespace,
		LabelSelector: l,
		MaxFound:      3,
	}
}

func (s *limitAvailModeChangeSuite) TestAvailModeUnchanged() {
	require := s.Require()
	smbShare := &sambaoperatorv1alpha1.SmbShare{}
	err := s.tc.TypedObjectClient().Get(
		context.TODO(), s.smbShareResource, smbShare)
	require.NoError(err)
	require.NotNil(smbShare.Annotations)
	require.Contains(smbShare.Annotations[backendAnnotation], s.expectBackend)
	if smbShare.Spec.Scaling == nil {
		smbShare.Spec.Scaling = &sambaoperatorv1alpha1.SmbShareScalingSpec{}
	}
	smbShare.Spec.Scaling.AvailabilityMode = s.nextMode
	smbShare.Spec.Scaling.MinClusterSize = 2

	err = s.tc.TypedObjectClient().Update(
		context.TODO(), smbShare)
	require.NoError(err)
	require.NoError(waitForPodExist(s), "smb server pod does not exist")
	require.NoError(waitForPodReady(s), "smb server pod is not ready")
	err = s.tc.TypedObjectClient().Get(
		context.TODO(), s.smbShareResource, smbShare)
	require.NoError(err)
	require.NotNil(smbShare.Annotations)
	require.Contains(smbShare.Annotations[backendAnnotation], s.expectBackend)
}

type scaleoutClusterSuite struct {
	suite.Suite

	fileSources      []kube.FileSource
	smbShareResource types.NamespacedName

	// cached values
	tc *kube.TestClient
}

func (s *scaleoutClusterSuite) SetupSuite() {
	// ensure the smbclient test pod exists
	require := s.Require()
	s.tc = kube.NewTestClient("")
	createSMBClientIfMissing(require, s.tc)
	createFromFiles(require, s.tc, s.fileSources)
	require.NoError(waitForPodExist(s), "smb server pod does not exist")
	require.NoError(waitForPodReady(s), "smb server pod is not ready")
}

func (s *scaleoutClusterSuite) TearDownSuite() {
	deleteFromFiles(s.Require(), s.tc, s.fileSources)
}

func (s *scaleoutClusterSuite) getTestClient() *kube.TestClient {
	return s.tc
}

func (s *scaleoutClusterSuite) getPodFetchOptions() kube.PodFetchOptions {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	return kube.PodFetchOptions{
		Namespace:     s.smbShareResource.Namespace,
		LabelSelector: l,
		MaxFound:      3,
	}
}

func (s *scaleoutClusterSuite) TestScaleoutClusterSuite() {
	require := s.Require()
	smbShare := &sambaoperatorv1alpha1.SmbShare{}
	err := s.tc.TypedObjectClient().Get(
		context.TODO(), s.smbShareResource, smbShare)
	require.NoError(err)

	// Increase Cluster Size by 1 and check result
	newClusterSize := smbShare.Spec.Scaling.MinClusterSize + 1
	smbShare.Spec.Scaling.MinClusterSize = newClusterSize
	err = s.tc.TypedObjectClient().Update(
		context.TODO(), smbShare)
	require.NoError(err)
	require.NoError(waitForPodExist(s), "smb server pod does not exist")
	require.NoError(waitForPodReady(s), "smb server pod is not ready")

	l, err := s.tc.Clientset().AppsV1().StatefulSets(s.smbShareResource.Namespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("samba-operator.samba.org/service=%s",
				s.smbShareResource.Name),
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
			{
				Path:       path.Join(testFilesDir, "smbshare1.yaml"),
				Namespace:  testNamespace,
				NameSuffix: "-bk",
			},
		},
		smbShareResource: types.NamespacedName{testNamespace, "tshare1-bk"},
		expectBackend:    "standard",
		nextMode:         "clustered",
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
			{
				Path:       path.Join(testFilesDir, "smbshare_ctdb1.yaml"),
				Namespace:  testNamespace,
				NameSuffix: "-bk",
			},
		},
		smbShareResource: types.NamespacedName{testNamespace, "cshare1-bk"},
		expectBackend:    "clustered",
		nextMode:         "standard",
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
			{
				Path:       path.Join(testFilesDir, "smbshare_ctdb1.yaml"),
				Namespace:  testNamespace,
				NameSuffix: "-soc",
			},
		},
		smbShareResource: types.NamespacedName{testNamespace, "cshare1-soc"},
	},
	)
}
