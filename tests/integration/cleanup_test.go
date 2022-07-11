//go:build integration
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

	commonSources   []kube.FileSource
	smbShareSources []kube.FileSource
	destNamespace   string
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

func (s *ShareCreateDeleteSuite) defaultContext() context.Context {
	return testContext()
}

func (s *ShareCreateDeleteSuite) SetupSuite() {
	s.testID = generateTestID()
	s.T().Logf("test ID: %s", s.testID)
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
	deleteFromFiles(s.defaultContext(), s.Require(), s.tc, s.commonSources)
}

func (s *ShareCreateDeleteSuite) getTestClient() *kube.TestClient {
	return s.tc
}

func (s *ShareCreateDeleteSuite) getPodFetchOptions() kube.PodFetchOptions {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.testShareName.Name)
	return kube.PodFetchOptions{
		Namespace:     s.destNamespace,
		LabelSelector: l,
		MaxFound:      s.maxPods,
		MinFound:      s.minPods,
	}
}

func (s *ShareCreateDeleteSuite) waitForNoSmbServices() error {
	ctx, cancel := context.WithDeadline(
		s.defaultContext(),
		time.Now().Add(waitForPodsTime))
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
			s.T().Logf("found samba server pod in namespace: %s",
				s.destNamespace)
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
		ctx     = s.defaultContext()
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
	var err error
	ctx := s.defaultContext()
	require := s.Require()
	existing := s.getCurrentResources()

	s.T().Log("creating prerequisite resources")
	createFromFiles(ctx, require, s.tc, s.commonSources)
	s.T().Log("creating smb share resource")
	names := createFromFilesWithSuffix(
		ctx,
		s.Require(),
		s.tc,
		s.smbShareSources,
		s.testID,
	)
	s.Require().Len(names, 1, "expected one smb share resource")
	s.testShareName = names[0]
	require.NoError(waitForPodExist(ctx, s))
	require.NoError(waitForAllPodReady(ctx, s))

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

	ctx2, cancel := context.WithDeadline(
		ctx,
		time.Now().Add(waitForPodsTime))
	defer cancel()

	// remove smbshare
	s.T().Log("removing smb share resource")
	smbShare := &sambaoperatorv1alpha1.SmbShare{}
	smbShare.Namespace = s.testShareName.Namespace
	smbShare.Name = s.testShareName.Name
	err = s.tc.TypedObjectClient().Delete(ctx2, smbShare)
	require.NoError(err)

	// wait for smbshare to go away
	s.T().Log("waiting for server resources to be removed")
	require.NoError(poll.TryUntil(ctx2, &poll.Prober{
		Cond: func() (bool, error) {
			smbShare := &sambaoperatorv1alpha1.SmbShare{}
			err := s.tc.TypedObjectClient().Get(
				ctx2, s.testShareName, smbShare)
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
	err = s.waitForNoSmbServices()
	require.NoError(err)

	s.T().Log("removing prerequisite resources")
	deleteFromFiles(ctx, require, s.tc, s.commonSources)

	s.requireResourcesDone(&existing)
}

func (s *ShareCreateDeleteSuite) requireResourcesDone(base *resourceSnapshot) {
	ctx, cancel := context.WithTimeout(s.defaultContext(), loginTestTimeout)
	defer cancel()
	s.Require().NoError(poll.TryUntil(ctx, &poll.Prober{
		RetryInterval: time.Second,
		Cond: func() (bool, error) {
			return s.checkResourcesDone(base), nil
		},
	}))
}

func (s *ShareCreateDeleteSuite) checkResourcesDone(
	base *resourceSnapshot) bool {
	curr := s.getCurrentResources()
	podsDiff := len(curr.pods.Items) - len(base.pods.Items)
	if podsDiff != 0 {
		s.T().Logf("%d pods still exist", podsDiff)
		return false
	}
	configMapsDiff := len(curr.configMaps.Items) - len(base.configMaps.Items)
	if configMapsDiff != 0 {
		s.T().Logf("%d configMaps still exist", configMapsDiff)
		return false
	}
	secretsDiff := len(curr.secrets.Items) - len(base.secrets.Items)
	if secretsDiff != 0 {
		s.T().Logf("%d secrets still exist", secretsDiff)
		return false
	}
	servicesDiff := len(curr.services.Items) - len(base.services.Items)
	if servicesDiff != 0 {
		s.T().Logf("%d services still exist", servicesDiff)
		return false
	}
	pvcsDiff := len(curr.pvcs.Items) - len(base.pvcs.Items)
	if pvcsDiff != 0 {
		s.T().Logf("%d pvcs still exist", pvcsDiff)
		return false
	}
	deploymentsDiff :=
		len(curr.deployments.Items) - len(base.deployments.Items)
	if deploymentsDiff != 0 {
		s.T().Logf("%d deployments still exist", deploymentsDiff)
		return false
	}
	statefulSetsDiff :=
		len(curr.statefulSets.Items) - len(base.statefulSets.Items)
	if statefulSetsDiff != 0 {
		s.T().Logf("%d statefulSets still exist", statefulSetsDiff)
		return false
	}
	return true
}

func init() {
	ns := testNamespace
	createDeleteTests := testRoot.ChildPriority("createDelete", 2)

	createDeleteTests.AddSuite("simple", &ShareCreateDeleteSuite{
		commonSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: ns,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: ns,
			},
		},
		smbShareSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "smbshare1.yaml"),
				Namespace: ns,
			},
		},
		destNamespace: ns,
		maxPods:       1,
		minPods:       1,
	},
	)

	createDeleteTests.AddSuite("domainMember", &ShareCreateDeleteSuite{
		commonSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "joinsecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig2.yaml"),
				Namespace: testNamespace,
			},
		},
		smbShareSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "smbshare2.yaml"),
				Namespace: testNamespace,
			},
		},
		destNamespace: ns,
		maxPods:       1,
		minPods:       1,
	},
	)

	// should we use a namespace other than default for this test?
	createDeleteTests.AddSuite("altNamespace", &ShareCreateDeleteSuite{
		commonSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: "default",
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: "default",
			},
		},
		smbShareSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "smbshare3.yaml"),
				Namespace: "default",
			},
		},
		destNamespace: "default",
		maxPods:       1,
		minPods:       1,
	},
	)

	if testClusteredShares {
		createDeleteTests.AddSuite("clustered", &ShareCreateDeleteSuite{
			commonSources: []kube.FileSource{
				{
					Path:      path.Join(testFilesDir, "userssecret1.yaml"),
					Namespace: ns,
				},
				{
					Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
					Namespace: ns,
				},
			},
			smbShareSources: []kube.FileSource{
				{
					Path:      path.Join(testFilesDir, "smbshare_ctdb1.yaml"),
					Namespace: ns,
				},
			},
			destNamespace: ns,
			maxPods:       3,
			minPods:       2,
		},
		)

		createDeleteTests.AddSuite("clusteredDomainMember", &ShareCreateDeleteSuite{
			commonSources: []kube.FileSource{
				{
					Path:      path.Join(testFilesDir, "joinsecret1.yaml"),
					Namespace: ns,
				},
				{
					Path:      path.Join(testFilesDir, "smbsecurityconfig2.yaml"),
					Namespace: ns,
				},
			},
			smbShareSources: []kube.FileSource{
				{
					Path:      path.Join(testFilesDir, "smbshare_ctdb2.yaml"),
					Namespace: ns,
				},
			},
			destNamespace: ns,
			maxPods:       3,
			minPods:       2,
		},
		)
	}
}
