// +build integration

package integration

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"time"

	"github.com/stretchr/testify/suite"

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

func allDeploySuites() map[string]suite.TestingSuite {
	m := map[string]suite.TestingSuite{}
	m["default"] = &DeploySuite{
		sharedConfigDir: path.Join(operatorConfigDir, "default"),
	}
	return m
}
