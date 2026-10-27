package commands_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

func TestFailingSetVerdictGroupsEveryRemainingMember(t *testing.T) {
	t.Parallel()

	item := &commands.BuildCodeItem{Tests: []*testrunner.FailingTest{
		{ID: "a", TestName: "pkg::TestA",
			FailureOutput: "=== RUN   TestA\nfile.go:12: shared failure\n--- FAIL: TestA (0.01s)"},
		{ID: "b", TestName: "pkg::TestB",
			FailureOutput: "=== RUN   TestB\nfile.go:42: shared failure\n--- FAIL: TestB (0.03s)"},
	}}

	verdict, err := commands.BuildCodeChecks()["tests-pass"].Run(item, checks.Input{})
	if err != nil || verdict.Pass || len(verdict.Context) != 1 || !strings.Contains(verdict.Context[0], "a, b") {
		t.Fatalf("grouped verdict: %+v, %v", verdict, err)
	}

	item.Tests[0].LastRunPassed = true
	if groups := item.Groups(); len(groups) != 1 || len(groups[0].IDs) != 1 || groups[0].IDs[0] != "b" {
		t.Fatalf("passing member must leave evidence: %+v", groups)
	}

	item.Tests[1].LastRunPassed = true

	verdict, err = commands.BuildCodeChecks()["tests-pass"].Run(item, checks.Input{})
	if err != nil || !verdict.Pass {
		t.Fatalf("whole set passed: %+v %v", verdict, err)
	}
}
