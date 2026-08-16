package bddgo

import "testing"

// World is what every scenario gets before its own state exists: the
// subtest it runs under, the scenario being run, and the suite the
// architectural spec assigned it to.
//
// Deliberately three fields. Anything else a suite needs belongs in the
// state type it hands to New — bddgo has no business knowing whether a
// suite drives a subprocess or a browser.
type World struct {
	T        *testing.T
	Scenario Scenario
	Suite    SuiteSpec
}
