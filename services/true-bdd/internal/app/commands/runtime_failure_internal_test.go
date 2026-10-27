package commands

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

const firstRuntimeTest = "fixture::TestOne"
const secondRuntimeTest = "fixture::TestTwo"
const firstRuntimeScenario = "one"
const normalizedSourceFailure = "    failure\n"

func TestNormalizeRuntimeFailure(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct{ name, output, testName, scenario, want string }{
		{"own name", "=== RUN   TestOne\n--- FAIL: TestOne (0.01s)\n",
			firstRuntimeTest, firstRuntimeScenario, "=== RUN   <test>\n--- FAIL: <test>\n"},
		{"qualified name", "fixture::TestOne: failed\n",
			firstRuntimeTest, firstRuntimeScenario, "<test>: failed\n"},
		{"file and line", "    path/scenario_test.go:42: failure\n",
			firstRuntimeTest, firstRuntimeScenario, normalizedSourceFailure},
		{"file line and column", "    /project/scenario_test.go:42:7: failure\n",
			firstRuntimeTest, firstRuntimeScenario, normalizedSourceFailure},
		{"source path with spaces", "    /some project/scenario_test.go:42: failure\n",
			firstRuntimeTest, firstRuntimeScenario, normalizedSourceFailure},
		{"error codes are evidence", "status:403: refused\n",
			firstRuntimeTest, firstRuntimeScenario, "status:403: refused\n"},
		{"URLs are evidence", "http://example.go:443: refused\n",
			firstRuntimeTest, firstRuntimeScenario, "http://example.go:443: refused\n"},
		{"elapsed", "--- FAIL: TestOne (123.456s)\n",
			firstRuntimeTest, firstRuntimeScenario, "--- FAIL: <test>\n"},
		{"scenario name header", "    scenario_test.go:42: one\n  expected 4, got 6\n",

			firstRuntimeTest, firstRuntimeScenario, "    <scenario>\n  expected 4, got 6\n"},
		{"assertion evidence", "expected one, got two; deadline (1.25s); other_test.go:12\n",

			firstRuntimeTest, firstRuntimeScenario, "expected one, got two; deadline (1.25s); other_test.go:12\n"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeRuntimeFailure(testCase.output, testCase.testName, testCase.scenario)
			if got != testCase.want {
				t.Fatalf("normalization = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestFailuresDifferingOnlyInTestNameShareKey(t *testing.T) {
	t.Parallel()

	first := normalizeRuntimeFailure(
		"=== RUN   TestFirst\nexpected 4, got 6\n--- FAIL: TestFirst (0.01s)\n", "fixture::TestFirst", "")

	second := normalizeRuntimeFailure(
		"=== RUN   TestSecond\nexpected 4, got 6\n--- FAIL: TestSecond (0.01s)\n", "fixture::TestSecond", "")
	if first != second {
		t.Fatalf("test names must not split one failure: %q != %q", first, second)
	}
}

func TestRuntimeFailuresGroupAcrossSelectionsAndKeepWholeRepresentative(t *testing.T) {
	t.Parallel()

	const evidence = "expected 4, got 6\n"

	report := runtimeFailureReport{}
	first := "=== RUN   TestFirst\n" + strings.Repeat(evidence, 1000)
	report.addSelection([]string{"fixture::TestFirst", "fixture::TestPassing"}, []string{"first", "passing"},
		[]*testrunner.FailingTest{{TestName: "fixture::TestFirst", FailureOutput: first}})
	report.addSelection([]string{"fixture::TestSecond"}, []string{"second"}, []*testrunner.FailingTest{
		{TestName: "fixture::TestSecond", FailureOutput: strings.ReplaceAll(first, "TestFirst", "TestSecond")},
	})

	got := strings.Join(report.render(), "\n")
	if strings.Count(got, "Failure affecting Scenarios:") != 1 ||
		!strings.Contains(got, "Failure affecting Scenarios: first, second\n"+first) {
		t.Fatalf("must group across selections and retain complete representative: %s", got)
	}

	if strings.Count(got, "Selection Scenarios:") != 2 ||
		strings.Contains(got, "Failure affecting Scenarios: first, passing") {
		t.Fatalf("selection and affected Scenarios differ: %s", got)
	}
}

func TestRuntimeFailuresKeepDistinctEvidenceAndAttributeSubtests(t *testing.T) {
	t.Parallel()

	report := runtimeFailureReport{}
	report.addSelection([]string{firstRuntimeTest, secondRuntimeTest}, []string{firstRuntimeScenario, "two"},
		[]*testrunner.FailingTest{
			{TestName: "fixture::TestOne/child", FailureOutput: "expected 4, got 6\n"},
			{TestName: secondRuntimeTest, FailureOutput: "expected 4, got 8\n"},
			{TestName: "fixture::TestOne/child", FailureOutput: "expected 4, got 6\n"},
		})

	got := strings.Join(report.render(), "\n")
	if strings.Count(got, "Failure affecting Scenarios:") != 2 ||
		!strings.Contains(got, "Scenarios: one\nexpected 4, got 6\n") ||
		!strings.Contains(got, "Scenarios: two\nexpected 4, got 8\n") {
		t.Fatalf("distinct evidence and unique affected ids: %s", got)
	}
}

func TestPackageFailureAffectsWholeSelection(t *testing.T) {
	t.Parallel()

	report := runtimeFailureReport{}
	report.addSelection([]string{firstRuntimeTest, secondRuntimeTest}, []string{firstRuntimeScenario, "two"},
		[]*testrunner.FailingTest{{TestName: "fixture::<build>", FailureOutput: "cannot compile\n"}})

	got := strings.Join(report.render(), "\n")
	if !strings.Contains(got, "Failure affecting Scenarios: one, two\ncannot compile\n") {
		t.Fatal(got)
	}
}
