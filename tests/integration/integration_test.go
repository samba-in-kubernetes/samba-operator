//go:build integration
// +build integration

package integration

import (
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type namedSuite struct {
	name      string
	testSuite suite.TestingSuite
}

// TestingGroup provides an interface to run groups of tests.
type TestingGroup interface {
	Name() string
	Run(*testing.T)
}

// Prioritized is an interface for checking a type's priority.
type Prioritized interface {
	Priority() int
}

// Shuffler is an interface for shuffling an object's contents.
type Shuffler interface {
	Shuffle()
}

// TestGroup contains other tests to run.
// Tests can be testify test suites or other child TestingGroups.
type TestGroup struct {
	name     string
	prio     int
	children []TestingGroup
	suites   []namedSuite
}

// Run all contained tests.
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

// Name of the test group.
func (tg *TestGroup) Name() string {
	return tg.name
}

// Priority of the test group. Lower value is higher priority.
func (tg *TestGroup) Priority() int {
	return tg.prio
}

// Child returns an new child test group with the provided name.
func (tg *TestGroup) Child(name string) *TestGroup {
	return tg.ChildPriority(name, 1)
}

// ChildPriority returns a new child test group with the provided
// name and priority value.
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

// AddSuite adds a test suite to the group.
func (tg *TestGroup) AddSuite(n string, s suite.TestingSuite) {
	tg.suites = append(tg.suites, namedSuite{name: n, testSuite: s})
}

// Shuffle the order of test suites and child test groups.
// If the suites or test groups meet the Shuffler interface the
// contents of the subobjects will be shuffled.
func (tg *TestGroup) Shuffle() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for _, c := range tg.children {
		if schild, ok := c.(Shuffler); ok {
			schild.Shuffle()
		}
	}
	r.Shuffle(len(tg.children), func(i, j int) {
		tg.children[i], tg.children[j] = tg.children[j], tg.children[i]
	})
	for _, s := range tg.suites {
		if ssuite, ok := s.testSuite.(Shuffler); ok {
			ssuite.Shuffle()
		}
	}
	r.Shuffle(len(tg.suites), func(i, j int) {
		tg.suites[i], tg.suites[j] = tg.suites[j], tg.suites[i]
	})
}

var testRoot *TestGroup = &TestGroup{}

// TestIntegration is the base test to start running our hierarchical
// integration test "super-suite".
func TestIntegration(t *testing.T) {
	if testShuffleOrder {
		t.Log("shuffling test order")
		testRoot.Shuffle()
	}
	testRoot.Run(t)
}
