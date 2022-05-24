//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func runSuiteTests(sm map[string]suite.TestingSuite) func(t *testing.T) {
	return func(t *testing.T) {
		for name, ts := range sm {
			t.Run(name, func(t *testing.T) {
				suite.Run(t, ts)
			})
		}
	}
}

func TestIntegration(t *testing.T) {
	t.Run("deploy", runSuiteTests(allDeploySuites()))
	t.Run("smbShares", runSuiteTests(allSmbShareSuites()))
	t.Run("createDelete", runSuiteTests(allShareCreateDeleteSuites()))
	t.Run("reconciliation", runSuiteTests(allReconcileSuites()))
}
