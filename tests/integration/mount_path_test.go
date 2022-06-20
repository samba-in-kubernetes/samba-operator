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

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

type MountPathSuite struct {
	suite.Suite

	auths                []smbclient.Auth
	commonSources        []kube.FileSource
	smbshareSetupSources []kube.FileSource
	smbshareSources      []kube.FileSource

	// tc is a TestClient instance
	tc *kube.TestClient

	// testID is a short unique test id, pseudo-randomly generated
	testID string
	// testShareName is the name of the SmbShare being tested by this
	// test instance
	testShareName types.NamespacedName
}

func (s *MountPathSuite) defaultContext() context.Context {
	return testContext()
}

func (s *MountPathSuite) waitForPods(labelPattern string) {
	require := s.Require()
	ctx, cancel := context.WithDeadline(
		s.defaultContext(),
		time.Now().Add(waitForPodsTime))
	defer cancel()
	opts := kube.PodFetchOptions{
		Namespace:     testNamespace,
		LabelSelector: labelPattern,
	}
	require.NoError(
		kube.WaitForAnyPodExists(ctx, s.tc, opts),
		"pod does not exist",
	)
	require.NoError(
		kube.WaitForAnyPodReady(ctx, s.tc, opts),
		"pod not ready",
	)
}

func (s *MountPathSuite) SetupSuite() {
	s.testID = generateTestID()
	s.tc = kube.NewTestClient("")
	ctx := s.defaultContext()
	require := s.Require()

	// Ensure smbclient is up and running
	createSMBClientIfMissing(ctx, require, s.tc)

	createFromFiles(ctx, require, s.tc, s.commonSources)
	names := createFromFilesWithSuffix(
		ctx,
		require,
		s.tc,
		s.smbshareSetupSources,
		s.testID,
	)
	require.Len(names, 1, "expected one smb share resource")
	setupName := names[0]
	// ensure the smbserver test pod exists and is ready
	setupLabel := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", setupName.Name)
	s.waitForPods(setupLabel)

	svcname := fmt.Sprintf("%s.%s.svc.cluster.local",
		setupName.Name,
		setupName.Namespace)
	share := smbclient.Share{
		Host: smbclient.Host(svcname),
		Name: setupName.Name,
	}

	// Create folders over smbclient
	smbclient := smbclient.MustPodExec(s.tc, testNamespace,
		"smbclient", "client")
	err := smbclient.CacheFlush(ctx)
	require.NoError(err)
	auth := s.auths[0]
	cmds := []string{
		"mkdir testmnt1",
		"mkdir testmnt2",
		"mkdir testmnt1/mnt1",
		"mkdir testmnt2/mnt2",
	}
	err = smbclient.Command(
		ctx,
		share,
		auth,
		cmds)
	require.NoError(err)

	// Delete the smbshare created
	deleteFromFilesWithSuffix(ctx, require, s.tc, s.smbshareSetupSources, s.testID)

	// Create smbshare with Spec.Storage.PVC.Path specified
	createFromFiles(ctx, require, s.tc, s.commonSources)
	names = createFromFilesWithSuffix(
		ctx,
		require,
		s.tc,
		s.smbshareSources,
		s.testID,
	)
	require.Len(names, 1, "expected one smb share resource")
	s.testShareName = names[0]
	lbl := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.testShareName.Name)
	s.waitForPods(lbl)
}

func (s *MountPathSuite) TearDownSuite() {
	ctx := s.defaultContext()
	require := s.Require()
	deleteFromFilesWithSuffix(ctx, require, s.tc, s.smbshareSources, s.testID)
	deleteFromFilesWithSuffix(ctx, require, s.tc, s.smbshareSetupSources, s.testID)
	deleteFromFiles(ctx, require, s.tc, s.commonSources)
}

func (s *MountPathSuite) TestMountPath() {
	ctx := s.defaultContext()
	require := s.Require()

	svcname := fmt.Sprintf("%s.%s.svc.cluster.local",
		s.testShareName.Name,
		s.testShareName.Namespace)
	share := smbclient.Share{
		Host: smbclient.Host(svcname),
		Name: s.testShareName.Name,
	}

	// Test if correct path mounted using smbclient
	smbclient := smbclient.MustPodExec(s.tc, testNamespace,
		"smbclient", "client")
	err := smbclient.CacheFlush(ctx)
	require.NoError(err)
	auth := s.auths[0]
	out, err := smbclient.CommandOutput(
		ctx,
		share,
		auth,
		[]string{"ls"})
	require.NoError(err)
	require.Contains(string(out), "mnt1")
}

func init() {
	mountPathTests := testRoot.ChildPriority("mountPath", 3)
	mountPathTests.AddSuite("default", &MountPathSuite{
		auths: []smbclient.Auth{
			{
				Username: "sambauser",
				Password: "1nsecurely",
			},
		},
		commonSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userpvc.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: testNamespace,
			},
		},
		smbshareSetupSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "smbsharepvc1.yaml"),
				Namespace: testNamespace,
			},
		},
		smbshareSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "smbsharepvc2.yaml"),
				Namespace: testNamespace,
			},
		},
	},
	)

	mountPathTests.AddSuite("permissions", &MountPathPermissionsSuite{
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
		smbshareSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "smbshare1.yaml"),
				Namespace: testNamespace,
			},
		},
	},
	)
}
