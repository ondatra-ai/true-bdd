package testkit_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

func TestRecordHintAnnotatesTheScenarioID(t *testing.T) {
	t.Parallel()

	scenario := testkit.Scenario{ID: "sample-001", Service: "sample", Steps: []bddgo.Step{
		{Keyword: "Given", Text: `the "different-fixture" project tree`},
	}}
	want := "record it with: go test -tags bdd -run '^TestSample001$' ./tests/sample/ " +
		"-mode=services:record,tests:record  # fixture: sample-001"

	if got := testkit.RecordHint(scenario); got != want {
		t.Fatalf("record hint = %q, want %q", got, want)
	}
}
