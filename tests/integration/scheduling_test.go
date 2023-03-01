//go:build integration
// +build integration

// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"fmt"
	"math/rand"
	"path"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	//"github.com/samba-in-kubernetes/samba-operator/tests/utils/poll"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

type CommonSelectorSuite struct {
	suite.Suite

	commonSources []kube.FileSource
	testAuths     []smbclient.Auth
	destNamespace string

	// cached values
	tc *kube.TestClient

	// testID is a short unique test id, pseudo-randomly generated
	testID string
	// labeledNode has the special label applied to it
	labeledNode string
	// numPods to create for our tests
	numPods int
}

func (s *CommonSelectorSuite) defaultContext() context.Context {
	ctx := testContext()
	if s.testID != "" {
		ctx = context.WithValue(ctx, TestIDKey, s.testID)
	}
	return ctx
}

func (s *CommonSelectorSuite) getTestClient() *kube.TestClient {
	return s.tc
}

func (s *CommonSelectorSuite) labelANode(ctx context.Context) {
	// apply a special label to only one node.
	// this test assumes that all linux-amd64 nodes in the test cluster can
	// run a samba pod. If not, this test may fail in unexpected ways.
	nodesList, err := s.tc.Clientset().CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/os=linux,kubernetes.io/arch=amd64,!node-role.kubernetes.io/control-plane",
	})
	s.Require().NoError(err)
	s.Require().Greater(len(nodesList.Items), 0)
	idx := rand.Intn(len(nodesList.Items))
	targetNode := nodesList.Items[idx]
	targetNode.Labels["mytestid"] = s.testID
	_, err = s.tc.Clientset().CoreV1().Nodes().Update(
		ctx, &targetNode, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.labeledNode = targetNode.Name
}

func (s *CommonSelectorSuite) unlabelNodes(ctx context.Context) {
	nodesList, err := s.tc.Clientset().CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("mytestid=%s", s.testID),
	})
	s.Require().NoError(err)
	s.Require().Greater(len(nodesList.Items), 0)
	for _, node := range nodesList.Items {
		delete(node.Labels, "mytestid")
		_, err = s.tc.Clientset().CoreV1().Nodes().Update(
			ctx, &node, metav1.UpdateOptions{})
		s.Require().NoError(err)
	}
}

func (s *CommonSelectorSuite) commonConfigName() string {
	return "schedtest-" + s.testID
}

func (s *CommonSelectorSuite) deleteSmbCommonConfig(ctx context.Context) {
	cc := &sambaoperatorv1alpha1.SmbCommonConfig{}
	nn := types.NamespacedName{
		Namespace: s.destNamespace,
		Name:      s.commonConfigName(),
	}
	err := s.tc.TypedObjectClient().Get(ctx, nn, cc)
	s.Require().NoError(err)
	err = s.tc.TypedObjectClient().Delete(ctx, cc)
	s.Require().NoError(err)
}

func (s *CommonSelectorSuite) smbShareTemplate(idx int) *sambaoperatorv1alpha1.SmbShare {
	sname := fmt.Sprintf("schedtest-%s-%d", s.testID, idx)
	return &sambaoperatorv1alpha1.SmbShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sname,
			Namespace: s.destNamespace,
		},
		Spec: sambaoperatorv1alpha1.SmbShareSpec{
			ShareName:      sname,
			ReadOnly:       false,
			Browseable:     true,
			SecurityConfig: "sharesec1",
			CommonConfig:   s.commonConfigName(),
			Storage: sambaoperatorv1alpha1.SmbShareStorageSpec{
				Pvc: &sambaoperatorv1alpha1.SmbSharePvcSpec{
					Name: sname,
					Spec: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
		},
	}
}

func (s *CommonSelectorSuite) createSmbShares(ctx context.Context) {
	for i := 0; i < s.numPods; i++ {
		smbShare := s.smbShareTemplate(i)
		err := s.tc.TypedObjectClient().Create(ctx, smbShare)
		s.Require().NoError(err)
	}
}

func (s *CommonSelectorSuite) deleteSmbShares(ctx context.Context) {
	for i := 0; i < s.numPods; i++ {
		smbShare := s.smbShareTemplate(i)
		err := s.tc.TypedObjectClient().Delete(ctx, smbShare)
		s.Require().NoError(err)
	}
}

func (s *CommonSelectorSuite) podLabelSelector() string {
	return "samba-operator.samba.org/service,samba-operator.samba.org/common-config-from=" + s.commonConfigName()
}

func (s *CommonSelectorSuite) getPodFetchOptions() kube.PodFetchOptions {
	return kube.PodFetchOptions{
		Namespace:     s.destNamespace,
		LabelSelector: s.podLabelSelector(),
		MaxFound:      s.numPods,
		MinFound:      1,
	}
}

type NodeSelectorSuite struct {
	CommonSelectorSuite
}

func (s *NodeSelectorSuite) createSmbCommonConfig(ctx context.Context) {
	cc := &sambaoperatorv1alpha1.SmbCommonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.commonConfigName(),
			Namespace: s.destNamespace,
			Labels: map[string]string{
				"mytestid": s.testID,
			},
		},
		Spec: sambaoperatorv1alpha1.SmbCommonConfigSpec{
			PodSettings: &sambaoperatorv1alpha1.SmbCommonConfigPodSettings{
				NodeSelector: map[string]string{
					"kubernetes.io/os":   "linux",
					"kubernetes.io/arch": "amd64",
					"mytestid":           s.testID,
				},
			},
		},
	}
	err := s.tc.TypedObjectClient().Create(ctx, cc)
	s.Require().NoError(err)
}

func (s *NodeSelectorSuite) SetupSuite() {
	s.testID = generateTestID()
	s.numPods = 4
	s.T().Logf("test ID: %s", s.testID)
	s.Require().NotEmpty(s.destNamespace)
	s.tc = kube.NewTestClient("")
	ctx := s.defaultContext()
	createFromFiles(ctx, s.Require(), s.tc, s.commonSources)

	s.labelANode(ctx)
	s.createSmbCommonConfig(ctx)
}

func (s *NodeSelectorSuite) TearDownSuite() {
	ctx := s.defaultContext()
	s.unlabelNodes(ctx)
	s.deleteSmbCommonConfig(ctx)
	deleteFromFiles(ctx, s.Require(), s.tc, s.commonSources)
}

func (s *NodeSelectorSuite) TestPodsRunOnLabeledNode() {
	ctx := s.defaultContext()
	require := s.Require()

	s.createSmbShares(ctx)
	require.NoError(waitForPodExist(ctx, s), "smb server pods do not exist")
	require.NoError(waitForAllPodReady(ctx, s), "smb server pods are not ready")

	podList, err := s.tc.Clientset().CoreV1().Pods(s.destNamespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: s.podLabelSelector()},
	)
	require.NoError(err)
	require.Greater(len(podList.Items), 0)
	for _, pod := range podList.Items {
		s.T().Logf("pod %s running on: %s", pod.Name, pod.Spec.NodeName)
		require.Equal(pod.Spec.NodeName, s.labeledNode, "pod not running on labeled node")
	}
}

func (s *NodeSelectorSuite) TearDownTest() {
	ctx := s.defaultContext()
	s.deleteSmbShares(ctx)
}

type AffinityBasedSelectorSuite struct {
	CommonSelectorSuite
}

func (s *AffinityBasedSelectorSuite) createSmbCommonConfig(ctx context.Context) {
	cc := &sambaoperatorv1alpha1.SmbCommonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.commonConfigName(),
			Namespace: s.destNamespace,
			Labels: map[string]string{
				"mytestid": s.testID,
			},
		},
		Spec: sambaoperatorv1alpha1.SmbCommonConfigSpec{
			PodSettings: &sambaoperatorv1alpha1.SmbCommonConfigPodSettings{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{{
							Weight: 10,
							Preference: corev1.NodeSelectorTerm{
								MatchExpressions: []corev1.NodeSelectorRequirement{{
									Key:      "mytestid",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{s.testID},
								}},
							},
						}},
					},
				},
			},
		},
	}
	err := s.tc.TypedObjectClient().Create(ctx, cc)
	s.Require().NoError(err)
}

func (s *AffinityBasedSelectorSuite) SetupSuite() {
	s.testID = generateTestID()
	s.numPods = 4
	s.T().Logf("test ID: %s", s.testID)
	s.Require().NotEmpty(s.destNamespace)
	s.tc = kube.NewTestClient("")
	ctx := s.defaultContext()
	createFromFiles(ctx, s.Require(), s.tc, s.commonSources)

	s.labelANode(ctx)
	s.createSmbCommonConfig(ctx)
}

func (s *AffinityBasedSelectorSuite) TearDownSuite() {
	ctx := s.defaultContext()
	s.unlabelNodes(ctx)
	s.deleteSmbCommonConfig(ctx)
	deleteFromFiles(ctx, s.Require(), s.tc, s.commonSources)
}

func (s *AffinityBasedSelectorSuite) TestPodsRunOnLabeledNode() {
	ctx := s.defaultContext()
	require := s.Require()

	s.createSmbShares(ctx)
	require.NoError(waitForPodExist(ctx, s), "smb server pods do not exist")
	require.NoError(waitForAllPodReady(ctx, s), "smb server pods are not ready")

	podList, err := s.tc.Clientset().CoreV1().Pods(s.destNamespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: s.podLabelSelector()},
	)
	require.NoError(err)
	require.Greater(len(podList.Items), 0)
	for _, pod := range podList.Items {
		s.T().Logf("pod %s running on: %s", pod.Name, pod.Spec.NodeName)
		require.Equal(pod.Spec.NodeName, s.labeledNode, "pod not running on labeled node")
	}
}

func (s *AffinityBasedSelectorSuite) TearDownTest() {
	ctx := s.defaultContext()
	s.deleteSmbShares(ctx)
}

func init() {
	schedulingTests := testRoot.ChildPriority("scheduling", 7)
	schedulingTests.AddSuite("NodeSelectorSuite", &NodeSelectorSuite{CommonSelectorSuite{
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
		destNamespace: testNamespace,
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
	}})
	schedulingTests.AddSuite("AffinityBasedSelectorSuite", &AffinityBasedSelectorSuite{CommonSelectorSuite{
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
		destNamespace: testNamespace,
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
	}})
}
