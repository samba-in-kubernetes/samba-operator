// +build integration

package integration

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

var waitForPodsTime = 20 * time.Second

type SmbShareSuite struct {
	suite.Suite

	fileSources      []kube.FileSource
	smbShareResource types.NamespacedName
	shareName        string
	testAuths        []smbclient.Auth
	destNamespace    string
	maxPods          int

	// cached values
	tc *kube.TestClient
}

func (s *SmbShareSuite) SetupSuite() {
	// ensure the smbclient test pod exists
	if s.destNamespace == "" {
		s.destNamespace = testNamespace
	}
	if s.maxPods == 0 {
		s.maxPods = 1
	}
	require := s.Require()
	s.tc = kube.NewTestClient("")
	for _, fs := range s.fileSources {
		_, err := s.tc.CreateFromFileIfMissing(
			context.TODO(),
			fs,
		)
		require.NoError(err)
	}
	require.NoError(s.waitForPodExist(), "smb server pod does not exist")
	require.NoError(s.waitForPodReady(), "smb server pod is not ready")
}

func (s *SmbShareSuite) TearDownSuite() {
	for _, fs := range s.fileSources {
		err := s.tc.DeleteResourceMatchingFile(
			context.TODO(),
			fs,
		)
		s.Require().NoError(err)
	}
}

func (s *SmbShareSuite) waitForPodExist() error {
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(waitForPodsTime))
	defer cancel()
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	return kube.WaitForAnyPodExists(
		ctx,
		s.tc,
		kube.PodFetchOptions{
			Namespace:     s.destNamespace,
			LabelSelector: l,
			MaxFound:      s.maxPods,
		})
}

func (s *SmbShareSuite) waitForPodReady() error {
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(60*time.Second))
	defer cancel()
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	return kube.WaitForAnyPodReady(
		ctx,
		s.tc,
		kube.PodFetchOptions{
			Namespace:     s.destNamespace,
			LabelSelector: l,
			MaxFound:      s.maxPods,
		})
}

func (s *SmbShareSuite) getPodIP() (string, error) {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	pods, err := s.tc.FetchPods(
		context.TODO(),
		kube.PodFetchOptions{
			Namespace:     s.destNamespace,
			LabelSelector: l,
			MaxFound:      s.maxPods,
		})
	if err != nil {
		return "", err
	}
	return pods[0].Status.PodIP, nil
}

func (s *SmbShareSuite) TestPodsReady() {
	s.Require().NoError(s.waitForPodReady())
}

func (s *SmbShareSuite) TestSmbShareServerGroup() {
	smbShare := &sambaoperatorv1alpha1.SmbShare{}
	err := s.tc.TypedObjectClient().Get(
		context.TODO(), s.smbShareResource, smbShare)
	s.Require().NoError(err)
	s.Require().Equal(s.smbShareResource.Name, smbShare.Name)
	s.Require().Equal(s.smbShareResource.Name, smbShare.Status.ServerGroup)
}

func (s *SmbShareSuite) TestShareAccessByIP() {
	ip, err := s.getPodIP()
	s.Require().NoError(err)
	shareAccessSuite := &ShareAccessSuite{
		share: smbclient.Share{
			Host: smbclient.Host(ip),
			Name: s.shareName,
		},
		auths: s.testAuths,
	}
	suite.Run(s.T(), shareAccessSuite)
}

func (s *SmbShareSuite) TestShareAccessByServiceName() {
	svcname := fmt.Sprintf("%s.%s.svc.cluster.local",
		s.smbShareResource.Name,
		s.destNamespace)
	shareAccessSuite := &ShareAccessSuite{
		share: smbclient.Share{
			Host: smbclient.Host(svcname),
			Name: s.shareName,
		},
		auths: s.testAuths,
	}
	suite.Run(s.T(), shareAccessSuite)
}

func (s *SmbShareSuite) TestShareEvents() {
	s.Require().NoError(s.waitForPodReady())

	// this unstructured stuff is just to get a UID for the SmbShare for event
	// filtering. Since the tests don't currently have a way to use a typed
	// interface for API access to SmbShare we take the lazy way out
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("samba-operator.samba.org/v1alpha1")
	u.SetKind("SmbShare")
	dc, err := s.tc.DynamicClientset(u)
	s.Require().NoError(err)
	u, err = dc.Namespace(s.smbShareResource.Namespace).Get(
		context.TODO(),
		s.smbShareResource.Name,
		metav1.GetOptions{})
	s.Require().NoError(err)

	l, err := s.tc.Clientset().CoreV1().Events(s.smbShareResource.Namespace).List(
		context.TODO(),
		metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.kind=SmbShare,involvedObject.name=%s,involvedObject.uid=%s", s.smbShareResource.Name, u.GetUID()),
		})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(l.Items), 1)
	numCreatedPVC := 0
	numCreatedInstance := 0
	for _, event := range l.Items {
		if event.Reason == "CreatedPersistentVolumeClaim" {
			numCreatedPVC++
		}
		if event.Reason == "CreatedDeployment" {
			numCreatedInstance++
		}
		if event.Reason == "CreatedStatefulSet" {
			numCreatedInstance++
		}
	}
	s.Require().Equal(1, numCreatedPVC)
	s.Require().Equal(1, numCreatedInstance)
}

type SmbShareWithDNSSuite struct {
	SmbShareSuite
}

func (s *SmbShareWithDNSSuite) TestShareAccessByDomainName() {
	// HACK: sleep for a short bit before running the smbclient command.  This
	// test works often but is flaky, and that appears due to the dns name not
	// always resolving. This hack adds a small delay to try and reduce the
	// test flakes. Obviously, a sleep is poor way to improve a test case. In
	// the future we should poll-with-a-timeout for the dns name, but that
	// needs to be done in-cluster/in-the-pod and that's a bunch of yak shaving
	// - so for now: a hack.
	time.Sleep(400 * time.Millisecond)

	dnsname := fmt.Sprintf("%s-cluster.domain1.sink.test",
		s.smbShareResource.Name)
	shareAccessSuite := &ShareAccessSuite{
		share: smbclient.Share{
			Host: smbclient.Host(dnsname),
			Name: s.shareName,
		},
		auths: s.testAuths,
	}
	suite.Run(s.T(), shareAccessSuite)
}

func (s *SmbShareWithDNSSuite) TestPodForDNSContainers() {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	pods, err := s.tc.FetchPods(
		context.TODO(),
		kube.PodFetchOptions{
			Namespace:     s.destNamespace,
			LabelSelector: l,
		},
	)
	s.Require().NoError(err)
	s.Require().Equal(4, len(pods[0].Spec.Containers))
	names := []string{}
	for _, cstatus := range pods[0].Status.ContainerStatuses {
		names = append(names, cstatus.Name)
		s.Require().True(cstatus.Ready, "container %s not ready", cstatus.Name)
	}
	s.Require().Contains(names, "dns-register")
	s.Require().Contains(names, "svc-watch")
}

type SmbShareWithExternalNetSuite struct {
	SmbShareSuite
}

func (s *SmbShareWithExternalNetSuite) TestServiceIsLoadBalancer() {
	lbl := fmt.Sprintf("samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	l, err := s.tc.Clientset().CoreV1().Services(s.destNamespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: lbl,
		},
	)
	s.Require().NoError(err)
	s.Require().Len(l.Items, 1)
	// our test environment does not require the k8s cluster to actually
	// support an external load balancer. All this test can do is check
	// IF LoadBalanacer was set.
	svc := l.Items[0]
	s.Require().Equal(
		corev1.ServiceTypeLoadBalancer,
		svc.Spec.Type,
	)
}

func allSmbShareSuites() map[string]suite.TestingSuite {
	m := map[string]suite.TestingSuite{}
	m["users1"] = &SmbShareSuite{
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
				Path:      path.Join(testFilesDir, "smbshare1.yaml"),
				Namespace: testNamespace,
			},
		},
		smbShareResource: types.NamespacedName{testNamespace, "tshare1"},
		shareName:        "My Share",
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
	}

	m["domainMember1"] = &SmbShareWithDNSSuite{SmbShareSuite{
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
		smbShareResource: types.NamespacedName{testNamespace, "tshare2"},
		shareName:        "My Kingdom",
		testAuths: []smbclient.Auth{{
			Username: "DOMAIN1\\bwayne",
			Password: "1115Rose.",
		}},
	}}

	// Test that the operator functions when the SmbShare resources are created
	// in a different ns (for example, "default").
	// IMPORTANT: the secrets MUST be in the same namespace as the pods.
	m["smbSharesInDefault"] = &SmbShareSuite{
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: "default",
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: "default",
			},
			{
				Path:      path.Join(testFilesDir, "smbshare3.yaml"),
				Namespace: "default",
			},
		},
		smbShareResource: types.NamespacedName{"default", "tshare3"},
		destNamespace:    "default",
		shareName:        "My Other Share",
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
	}

	m["smbSharesExternal"] = &SmbShareWithExternalNetSuite{SmbShareSuite{
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "commonconfig1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbshare4.yaml"),
				Namespace: testNamespace,
			},
		},
		smbShareResource: types.NamespacedName{testNamespace, "tshare4"},
		shareName:        "Since When",
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
	}}

	return m
}

func init() {
	utilruntime.Must(sambaoperatorv1alpha1.AddToScheme(kube.TypedClientScheme))
}
