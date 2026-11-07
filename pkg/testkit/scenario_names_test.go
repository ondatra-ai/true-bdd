package testkit_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

func TestFixtureNameUsesTheFirstMatchingStepOrTheScenarioID(t *testing.T) {
	t.Parallel()

	scenario := testkit.Scenario{ID: "fallback", Steps: []bddgo.Step{
		{Keyword: "When", Text: "fixture first"},
		{Keyword: "Given", Text: "fixture second"},
	}}

	if got := testkit.FixtureName(scenario, `^fixture (.+)$`); got != "first" {
		t.Fatalf("first matching step = %q, want first", got)
	}

	for _, patterns := range [][]string{nil, {`^absent (.+)$`}} {
		if got := testkit.FixtureName(scenario, patterns...); got != "fallback" {
			t.Errorf("unmatched pattern %v = %q, want fallback", patterns, got)
		}
	}
}
