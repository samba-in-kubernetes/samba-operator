//go:build integration
// +build integration

package integration

import (
	"context"
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
	s.Require().NoError(err, "kustomize command failed to start")
	_, err = s.tc.CreateFromFileIfMissing(
		context.TODO(),
		kube.DirectSource{
			Source:    stdout,
			Namespace: testNamespace,
		},
	)
	s.Require().NoError(err, "CreateFromFileIfMissing failed")
	err = cmd.Wait()
	s.Require().NoError(err, "kustomize command failed")
}

func (s DeploySuite) TestOperatorReady() {
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(90*time.Second))
	defer cancel()
	l := "control-plane=controller-manager"
	err := kube.WaitForAnyPodExists(
		ctx,
		s.tc,
		kube.PodFetchOptions{
			Namespace:     testNamespace,
			LabelSelector: l,
		},
	)
	s.Require().NoError(err)
	err = kube.WaitForAnyPodReady(
		ctx,
		s.tc,
		kube.PodFetchOptions{
			Namespace:     testNamespace,
			LabelSelector: l,
		},
	)
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

func init() {
	testRoot.AddSuite("deploy", &DeploySuite{
		sharedConfigDir: path.Join(operatorConfigDir, "default"),
	})
}
