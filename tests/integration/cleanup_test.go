// +build integration

package integration

import (
	"context"
	"fmt"
	"math"
	"path"
	"time"

	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/poll"
)

type resourceSnapshot struct {
	pods       *corev1.PodList
	services   *corev1.ServiceList
	secrets    *corev1.SecretList
	configMaps *corev1.ConfigMapList
	pvcs       *corev1.PersistentVolumeClaimList

	deployments  *appsv1.DeploymentList
	statefulSets *appsv1.StatefulSetList
}

type ShareCreateDeleteSuite struct {
	suite.Suite

	fileSources      []kube.FileSource
	smbShareResource types.NamespacedName
	destNamespace    string
	maxPods          int
	minPods          int

	// cached values
	tc *kube.TestClient
}

func (s *ShareCreateDeleteSuite) SetupSuite() {
	s.tc = kube.NewTestClient("")
}

func (s *ShareCreateDeleteSuite) SetupTest() {
	// not all our test cases wait for all their resources to be cleaned
	// up. This setup func tries to wait until that has happened before
	// we execute our tests.
	err := s.waitForNoSmbServices()
	if err != nil {
		// don't fail, let the test do that, but do warn in case something
		// very odd happened
		fmt.Println("error: waiting for older pods to be cleaned:", err)
	}
}

func (s *ShareCreateDeleteSuite) TearDownSuite() {
	deleteFromFiles(s.Require(), s.tc, s.fileSources)
}

func (s *ShareCreateDeleteSuite) getTestClient() *kube.TestClient {
	return s.tc
}

func (s *ShareCreateDeleteSuite) getPodFetchOptions() kube.PodFetchOptions {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	return kube.PodFetchOptions{
		Namespace:     s.destNamespace,
		LabelSelector: l,
		MaxFound:      s.maxPods,
		MinFound:      s.minPods,
	}
}

func (s *ShareCreateDeleteSuite) waitForNoSmbServices() error {
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(60*time.Second))
	defer cancel()
	err := poll.TryUntil(ctx, &poll.Prober{
		Cond: func() (bool, error) {
			// Looking for old stuff...
			l := "samba-operator.samba.org/service"
			// only set max pods since were waiting for "drain"
			_, err := s.tc.FetchPods(
				ctx,
				kube.PodFetchOptions{
					Namespace:     s.destNamespace,
					LabelSelector: l,
					MaxFound:      math.MaxInt32,
				})
			if err == kube.ErrNoMatchingPods {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			return false, nil
		},
	})
	return err
}

func (s *ShareCreateDeleteSuite) getCurrentResources() resourceSnapshot {
	var (
		err     error
		rs      resourceSnapshot
		opts    metav1.ListOptions
		ctx     = context.TODO()
		require = s.Require()
	)

	rs.pods, err = s.tc.Clientset().CoreV1().
		Pods(s.destNamespace).List(ctx, opts)
	require.NoError(err)
	rs.services, err = s.tc.Clientset().CoreV1().
		Services(s.destNamespace).List(ctx, opts)
	require.NoError(err)
	rs.secrets, err = s.tc.Clientset().CoreV1().
		Secrets(s.destNamespace).List(ctx, opts)
	require.NoError(err)
	rs.configMaps, err = s.tc.Clientset().CoreV1().
		ConfigMaps(s.destNamespace).List(ctx, opts)
	require.NoError(err)
	rs.pvcs, err = s.tc.Clientset().CoreV1().
		PersistentVolumeClaims(s.destNamespace).List(ctx, opts)
	require.NoError(err)
	rs.deployments, err = s.tc.Clientset().AppsV1().
		Deployments(s.destNamespace).List(ctx, opts)
	require.NoError(err)
	rs.statefulSets, err = s.tc.Clientset().AppsV1().
		StatefulSets(s.destNamespace).List(ctx, opts)
	require.NoError(err)

	return rs
}

func (s *ShareCreateDeleteSuite) TestCreateAndDelete() {
	require := s.Require()
	existing := s.getCurrentResources()

	createFromFiles(require, s.tc, s.fileSources)
	require.NoError(waitForPodExist(s))
	require.NoError(waitForPodReady(s))

	rs1 := s.getCurrentResources()
	require.Greater(len(rs1.pods.Items), len(existing.pods.Items))
	require.Greater(len(rs1.configMaps.Items), len(existing.configMaps.Items))
	require.Greater(len(rs1.secrets.Items), len(existing.secrets.Items))
	require.Greater(len(rs1.services.Items), len(existing.services.Items))
	require.Greater(len(rs1.pvcs.Items), len(existing.pvcs.Items))
	require.GreaterOrEqual(
		len(rs1.deployments.Items), len(existing.deployments.Items))
	require.GreaterOrEqual(
		len(rs1.statefulSets.Items), len(existing.statefulSets.Items))

	deleteFromFiles(require, s.tc, s.fileSources)

	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(60*time.Second))
	defer cancel()
	// wait for smbshare to go away
	require.NoError(poll.TryUntil(ctx, &poll.Prober{
		Cond: func() (bool, error) {
			smbShare := &sambaoperatorv1alpha1.SmbShare{}
			err := s.tc.TypedObjectClient().Get(
				ctx, s.smbShareResource, smbShare)
			if err == nil {
				// found is false ... we're waiting for it to go away
				return false, nil
			}
			if kerrors.IsNotFound(err) {
				// nothing was found
				return true, nil
			}
			return false, err
		},
	}))
	// wait for pods to go away
	err := s.waitForNoSmbServices()
	require.NoError(err)

	rs2 := s.getCurrentResources()
	require.Equal(len(rs2.pods.Items), len(existing.pods.Items))
	require.Equal(len(rs2.configMaps.Items), len(existing.configMaps.Items))
	require.Equal(len(rs2.secrets.Items), len(existing.secrets.Items))
	require.Equal(len(rs2.services.Items), len(existing.services.Items))
	require.Equal(len(rs2.pvcs.Items), len(existing.pvcs.Items))
	require.Equal(
		len(rs2.deployments.Items), len(existing.deployments.Items))
	require.Equal(
		len(rs2.statefulSets.Items), len(existing.statefulSets.Items))
}

func allShareCreateDeleteSuites() map[string]suite.TestingSuite {
	m := map[string]suite.TestingSuite{}
	ns := testNamespace

	m["simple"] = &ShareCreateDeleteSuite{
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: ns,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: ns,
			},
			{
				Path:      path.Join(testFilesDir, "smbshare1.yaml"),
				Namespace: ns,
			},
		},
		destNamespace:    ns,
		smbShareResource: types.NamespacedName{ns, "tshare1"},
		maxPods:          1,
		minPods:          1,
	}
	m["domainMember"] = &ShareCreateDeleteSuite{
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "joinsecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig2.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbshare2.yaml"),
				Namespace: testNamespace,
			},
		},
		destNamespace:    ns,
		smbShareResource: types.NamespacedName{testNamespace, "tshare2"},
		maxPods:          1,
		minPods:          1,
	}

	return m
}
