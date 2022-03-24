// +build integration

package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/types"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

type MountPathSuite struct {
	suite.Suite

	auths                   []smbclient.Auth
	commonSources           []kube.FileSource
	smbshareSetupSources    []kube.FileSource
	smbshareSources         []kube.FileSource
	smbShareSetupResource   types.NamespacedName
	setupServerLabelPattern string
	smbShareResource        types.NamespacedName
	serverLabelPattern      string
	tc                      *kube.TestClient
}

func (s *MountPathSuite) waitForPods(labelPattern string) {
	require := s.Require()
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(waitForPodsTime))
	defer cancel()
	opts := kube.PodFetchOptions{
		Namespace:     testNamespace,
		LabelSelector: labelPattern,
	}
	require.NoError(
		kube.WaitForAnyPodExists(ctx, kube.NewTestClient(""), opts),
		"pod does not exist",
	)
	require.NoError(
		kube.WaitForAnyPodReady(ctx, kube.NewTestClient(""), opts),
		"pod not ready",
	)
}

func (s *MountPathSuite) SetupSuite() {
	s.tc = kube.NewTestClient("")
	require := s.Require()

	// Ensure smbclient is up and running
	createSMBClientIfMissing(require, s.tc)

	createFromFiles(require, s.tc, append(s.commonSources, s.smbshareSetupSources...))
	// ensure the smbserver test pod exists and is ready
	s.waitForPods(s.setupServerLabelPattern)

	svcname := fmt.Sprintf("%s.%s.svc.cluster.local",
		s.smbShareSetupResource.Name,
		s.smbShareSetupResource.Namespace)
	share := smbclient.Share{
		Host: smbclient.Host(svcname),
		Name: s.smbShareSetupResource.Name,
	}

	// Create folders over smbclient
	smbclient := smbclient.MustPodExec(s.tc, testNamespace,
		"smbclient", "client")
	err := smbclient.CacheFlush(context.TODO())
	require.NoError(err)
	auth := s.auths[0]
	cmds := []string{
		"mkdir testmnt1",
		"mkdir testmnt2",
		"mkdir testmnt1/mnt1",
		"mkdir testmnt2/mnt2",
	}
	err = smbclient.Command(
		context.TODO(),
		share,
		auth,
		cmds)
	require.NoError(err)

	// Delete the smbshare created
	deleteFromFiles(require, s.tc, s.smbshareSetupSources)

	// Create smbshare with Spec.Storage.PVC.Path specified
	createFromFiles(require, s.tc, append(s.commonSources, s.smbshareSources...))
	s.waitForPods(s.serverLabelPattern)
}

func (s *MountPathSuite) TearDownSuite() {
	deleteFromFiles(s.Require(), s.tc, s.smbshareSetupSources)
	deleteFromFiles(s.Require(), s.tc, s.smbshareSources)
	deleteFromFiles(s.Require(), s.tc, s.commonSources)
}

func (s *MountPathSuite) TestMountPath() {
	require := s.Require()

	svcname := fmt.Sprintf("%s.%s.svc.cluster.local",
		s.smbShareResource.Name,
		s.smbShareResource.Namespace)
	share := smbclient.Share{
		Host: smbclient.Host(svcname),
		Name: s.smbShareResource.Name,
	}

	// Test if correct path mounted using smbclient
	smbclient := smbclient.MustPodExec(s.tc, testNamespace,
		"smbclient", "client")
	err := smbclient.CacheFlush(context.TODO())
	require.NoError(err)
	auth := s.auths[0]
	out, err := smbclient.CommandOutput(
		context.TODO(),
		share,
		auth,
		[]string{"ls"})
	require.NoError(err)
	require.Contains(string(out), "mnt1")
}
