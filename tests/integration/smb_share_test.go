// +build integration

package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

type SmbShareSuite struct {
	suite.Suite

	fileSources          []string
	smbShareResourceName string
	shareName            string
	testAuths            []smbclient.Auth

	// cached values
	tc *kube.TestClient
}

func (s *SmbShareSuite) SetupSuite() {
	// ensure the smbclient test pod exists
	require := s.Require()
	s.tc = kube.NewTestClient("")
	for _, f := range s.fileSources {
		_, err := s.tc.CreateFromFileIfMissing(
			context.TODO(),
			kube.FileSource{
				Path:      f,
				Namespace: testNamespace,
			},
		)
		require.NoError(err)
	}
	require.NoError(s.waitForPodExist(), "smb server pod does not exist")
	require.NoError(s.waitForPodReady(), "smb server pod is not ready")
}

func (s *SmbShareSuite) TearDownSuite() {
	for _, f := range s.fileSources {
		err := s.tc.DeleteResourceMatchingFile(
			context.TODO(),
			kube.FileSource{
				Path:      f,
				Namespace: testNamespace,
			},
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
		fmt.Sprintf("samba-operator.samba.org/service=%s", s.smbShareResourceName),
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
		fmt.Sprintf("samba-operator.samba.org/service=%s", s.smbShareResourceName),
		testNamespace)
}

func (s *SmbShareSuite) getPodIP() (string, error) {
	pod, err := s.tc.GetPodByLabel(
		context.TODO(),
		fmt.Sprintf("samba-operator.samba.org/service=%s", s.smbShareResourceName),
		testNamespace)
	if err != nil {
		return "", err
	}
	return pod.Status.PodIP, nil
}

func (s *SmbShareSuite) TestPodsReady() {
	s.Require().NoError(s.waitForPodReady())
}

func (s *SmbShareSuite) TestShareAccess() {
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

func allSmbShareSuites() map[string]suite.TestingSuite {
	m := map[string]suite.TestingSuite{}
	m["users1"] = &SmbShareSuite{
		fileSources: []string{
			"../files/smbsecurityconfig1.yaml",
			"../files/smbshare1.yaml",
		},
		smbShareResourceName: "tshare1",
		shareName:            "My Share",
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
	}

	m["domainMember1"] = &SmbShareSuite{
		fileSources: []string{
			"../files/smbsecurityconfig2.yaml",
			"../files/smbshare2.yaml",
		},
		smbShareResourceName: "tshare2",
		shareName:            "My Kingdom",
		testAuths: []smbclient.Auth{{
			Username: "DOMAIN1\\bwayne",
			Password: "1115Rose.",
		}},
	}

	return m
}
