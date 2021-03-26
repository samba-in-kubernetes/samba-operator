// +build integration

package integration

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"time"

	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
)

type DeploySuite struct {
	suite.Suite

	sharedConfigDir string

	// cached values
	tc *kube.TestClient
}

// SetupSuite sets up (deploys) the operator.
func (s *DeploySuite) SetupSuite() {
	s.tc = kube.NewTestClient("")
	s.createKustomized(s.sharedConfigDir)
}

func (s DeploySuite) createKustomized(dir string) {
	cmd := exec.Command(kustomizeCmd, "build", dir)
	stdout, err := cmd.StdoutPipe()
	s.Require().NoError(err)
	err = cmd.Start()
	s.Require().NoError(err)
	_, err = s.tc.CreateFromFileIfMissing(
		context.TODO(),
		kube.DirectSource{
			Source:    stdout,
			Namespace: testNamespace,
		},
	)
	s.Require().NoError(err)
	err = cmd.Wait()
	s.Require().NoError(err)
}

func (s DeploySuite) TestOperatorReady() {
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(90*time.Second))
	defer cancel()
	err := kube.WaitForPodExistsByLabel(
		ctx,
		s.tc,
		fmt.Sprintf("control-plane=controller-manager"),
		testNamespace)
	s.Require().NoError(err)
	err = kube.WaitForPodReadyByLabel(
		ctx,
		s.tc,
		fmt.Sprintf("control-plane=controller-manager"),
		testNamespace)
	s.Require().NoError(err)
}

// TestImageAndTag is an optional test that verifies the container
// image and tag for the deployment is what was specified for the
// test.
func (s DeploySuite) TestImageAndTag() {
	if testExpectedImage == "" {
		s.T().Skip("testExpectedImage variable unset")
		return
	}
	deploy, err := s.tc.Clientset().AppsV1().Deployments(testNamespace).Get(
		context.TODO(),
		"samba-operator-controller-manager",
		metav1.GetOptions{})
	require := s.Require()
	require.NoError(err)
	var ctrImage string
	for _, ctr := range deploy.Spec.Template.Spec.Containers {
		if ctr.Name == "manager" {
			ctrImage = ctr.Image
			break
		}
	}
	require.Equal(testExpectedImage, ctrImage)
}

func allDeploySuites() map[string]suite.TestingSuite {
	m := map[string]suite.TestingSuite{}
	m["default"] = &DeploySuite{
		sharedConfigDir: path.Join(operatorConfigDir, "default"),
	}
	return m
}
