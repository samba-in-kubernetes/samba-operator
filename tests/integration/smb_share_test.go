// +build integration

package integration

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

type SmbShareSuite struct {
	suite.Suite

	fileSources      []kube.FileSource
	smbShareResource types.NamespacedName
	shareName        string
	testAuths        []smbclient.Auth

	// cached values
	tc *kube.TestClient
}

func (s *SmbShareSuite) SetupSuite() {
	// ensure the smbclient test pod exists
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
		time.Now().Add(10*time.Second))
	defer cancel()
	return kube.WaitForPodExistsByLabel(
		ctx,
		s.tc,
		fmt.Sprintf("samba-operator.samba.org/service=%s", s.smbShareResource.Name),
		testNamespace)
}

func (s *SmbShareSuite) waitForPodReady() error {
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(60*time.Second))
	defer cancel()
	return kube.WaitForPodReadyByLabel(
		ctx,
		s.tc,
		fmt.Sprintf("samba-operator.samba.org/service=%s", s.smbShareResource.Name),
		testNamespace)
}

func (s *SmbShareSuite) getPodIP() (string, error) {
	pod, err := s.tc.GetPodByLabel(
		context.TODO(),
		fmt.Sprintf("samba-operator.samba.org/service=%s", s.smbShareResource.Name),
		testNamespace)
	if err != nil {
		return "", err
	}
	return pod.Status.PodIP, nil
}

func (s *SmbShareSuite) TestPodsReady() {
	s.Require().NoError(s.waitForPodReady())
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
		testNamespace)
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
	numCreatedDeployment := 0
	for _, event := range l.Items {
		if event.Reason == "CreatedPersistentVolumeClaim" {
			numCreatedPVC++
		}
		if event.Reason == "CreatedDeployment" {
			numCreatedDeployment++
		}
	}
	s.Require().Equal(1, numCreatedPVC)
	s.Require().Equal(1, numCreatedDeployment)
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

	m["domainMember1"] = &SmbShareSuite{
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
	}

	// Test that the operator functions when the SmbShare resources are created
	// in a different ns (for example, "default").
	// IMPORTANT: the secrets MUST be in the same namespace as the pods.
	m["smbSharesInDefault"] = &SmbShareSuite{
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
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
		shareName:        "My Other Share",
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
	}

	return m
}
