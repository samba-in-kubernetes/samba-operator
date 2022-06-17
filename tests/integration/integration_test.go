//go:build integration
// +build integration

package integration

import (
	"sort"
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

type Prioritized interface {
	Priority() int
}

type TestGroup struct {
	name     string
	prio     int
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

func (tg *TestGroup) Priority() int {
	return tg.prio
}

func (tg *TestGroup) Child(name string) *TestGroup {
	return tg.ChildPriority(name, 1)
}

func (tg *TestGroup) ChildPriority(name string, prio int) *TestGroup {
	child := &TestGroup{name: name, prio: prio}
	tg.children = append(tg.children, child)
	sort.SliceStable(tg.children, func(i, j int) bool {
		pi := 0
		pj := 0
		if p, ok := tg.children[i].(Prioritized); ok {
			pi = p.Priority()
		}
		if p, ok := tg.children[j].(Prioritized); ok {
			pj = p.Priority()
		}
		return pi < pj
	})
	return child
}

func (tg *TestGroup) AddSuite(n string, s suite.TestingSuite) {
	tg.suites = append(tg.suites, namedSuite{name: n, testSuite: s})
}

var testRoot *TestGroup = &TestGroup{}

func TestIntegration(t *testing.T) {
	testRoot.Run(t)
}
