package bddgo

import "testing"

// World is what every scenario gets before its own state exists: the
// subtest it runs under, the scenario being run, and the suite the
// architectural spec assigned it to. Deliberately just these three fields.
type World struct {
	T        *testing.T
	Scenario Scenario
	Suite    SuiteSpec
}
