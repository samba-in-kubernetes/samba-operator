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

type namedSuite struct {
	name      string
	testSuite suite.TestingSuite
}

type TestingGroup interface {
	Name() string
	Run(*testing.T)
}

type TestGroup struct {
	name     string
	children []TestingGroup
	suites   []namedSuite
}

func (tg *TestGroup) Run(t *testing.T) {
	for _, s := range tg.suites {
		t.Run(s.name, func(t *testing.T) {
			suite.Run(t, s.testSuite)
		})
	}
	for _, child := range tg.children {
		t.Run(child.Name(), child.Run)
	}
}

func (tg *TestGroup) Name() string {
	return tg.name
}

func (tg *TestGroup) Child(name string) *TestGroup {
	child := &TestGroup{name: name}
	tg.children = append(tg.children, child)
	return child
}

func (tg *TestGroup) AddSuite(n string, s suite.TestingSuite) {
	tg.suites = append(tg.suites, namedSuite{name: n, testSuite: s})
}

var testRoot *TestGroup = &TestGroup{}

func TestIntegration(t *testing.T) {
	t.Run("deploy", runSuiteTests(allDeploySuites()))
	t.Run("smbShares", runSuiteTests(allSmbShareSuites()))
	t.Run("createDelete", runSuiteTests(allShareCreateDeleteSuites()))
	t.Run("reconciliation", runSuiteTests(allReconcileSuites()))
}
