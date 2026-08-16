package bddgo

import (
	"errors"
	"strings"
	"testing"
)

// firstScenario is the id these tests expect selection to return.
const firstScenario = "E2E-001"

// specDoc is a one-suite architectural spec: the cli suite, exercising
// the bdd-cli service.
const specDoc = `
architecture:
  testing:
    suites:
      - name: cli
        service: bdd-cli
        path: tests/bdd-cli
        framework: go-test
  services:
    - name: bdd-cli
      path: services/bdd-cli
      language: go
`

// registryDoc has one scenario for the cli suite's service and one for
// somebody else's, so a selection bug shows up as a count.
const registryDoc = `
scenarios:
  E2E-001:
    description: mine
    service: bdd-cli
    user_stories: []
    merged_steps:
      given: [a tree]
      when: [the CLI runs]
  E2E-002:
    description: not mine
    service: bdd-web
    user_stories: []
    merged_steps:
      when: [the browser opens]
`

// testState is deliberately empty: these tests exercise selection,
// binding and reporting, none of which touch what a suite keeps.
type testState struct{}

func newSuite(t *testing.T) *Suite[testState] {
	t.Helper()

	suite := New[testState](t, Options{
		Registry:     writeDoc(t, "scenarios.yaml", registryDoc),
		Architecture: writeDoc(t, "architecture.yaml", specDoc),
		Suite:        "cli",
	})
	suite.Init(func(*World) (*testState, error) { return &testState{}, nil })

	return suite
}

// A suite runs the scenarios its own service owns and nobody else's —
// the join is one line of the architectural spec, not a constant in the
// test binary.
func TestSuiteSelectsByService(t *testing.T) {
	t.Parallel()

	scenarios, err := newSuite(t).Scenarios()
	if err != nil {
		t.Fatalf("Scenarios: %v", err)
	}

	if len(scenarios) != 1 || scenarios[0].ID != firstScenario {
		t.Fatalf("want only E2E-001, got %+v", scenarios)
	}
}

// The spec is the authority on which suites exist, and a name it does
// not declare is refused with the list it does — the mistake is nearly
// always a typo and the fix is nearly always in that list.
func TestNewRefusesUndeclaredSuite(t *testing.T) {
	t.Parallel()

	_, err := LoadSuiteSpec(writeDoc(t, "architecture.yaml", specDoc), "clii")
	if !errors.Is(err, ErrSuiteNotDeclared) {
		t.Fatalf("want ErrSuiteNotDeclared, got %v", err)
	}

	if !strings.Contains(err.Error(), "declared: cli") {
		t.Errorf("the refusal must name what IS declared, got %q", err)
	}
}

// Every step of every owned scenario must bind, and Undefined answers
// that without running a thing — which is what makes it affordable for
// `build tests` to ask on every walk.
func TestUndefinedListsEveryUnboundStep(t *testing.T) {
	t.Parallel()

	suite := newSuite(t)
	suite.Step(`^a tree$`, func(*testState, []string) error { return nil })

	gaps, err := suite.Undefined()
	if err != nil {
		t.Fatalf("Undefined: %v", err)
	}

	steps := gaps[firstScenario]
	if len(steps) != 1 || steps[0].Text != "the CLI runs" {
		t.Fatalf("want the unbound When step, got %+v", gaps)
	}
}

// With every step bound there is no gap, and the report says so by
// being empty rather than by omitting the scenario.
func TestUndefinedIsEmptyWhenEveryStepBinds(t *testing.T) {
	t.Parallel()

	suite := newSuite(t)
	suite.Step(`^a tree$`, func(*testState, []string) error { return nil })
	suite.Step(`^the CLI runs$`, func(*testState, []string) error { return nil })

	gaps, err := suite.Undefined()
	if err != nil {
		t.Fatalf("Undefined: %v", err)
	}

	if len(gaps) != 0 {
		t.Fatalf("want no gaps, got %+v", gaps)
	}
}

// Two definitions matching one step is refused rather than resolved by
// registration order: which one runs would otherwise depend on the order
// of two lines nobody reads while writing scenarios.
func TestResolveRefusesAmbiguousStep(t *testing.T) {
	t.Parallel()

	suite := newSuite(t)
	suite.Step(`^a tree$`, func(*testState, []string) error { return nil })
	suite.Step(`^a (.+)$`, func(*testState, []string) error { return nil })
	suite.Step(`^the CLI runs$`, func(*testState, []string) error { return nil })

	_, err := suite.Undefined()
	if !errors.Is(err, ErrAmbiguousStep) {
		t.Fatalf("want ErrAmbiguousStep, got %v", err)
	}
}

// A suite whose service no scenario names runs nothing and would pass
// trivially, so it is refused instead.
func TestScenariosRefusesEmptySelection(t *testing.T) {
	t.Parallel()

	suite := New[testState](t, Options{
		Registry: writeDoc(t, "scenarios.yaml", `
scenarios:
  E2E-002:
    description: not mine
    service: bdd-web
    user_stories: []
    merged_steps:
      when: [the browser opens]
`),
		Architecture: writeDoc(t, "architecture.yaml", specDoc),
		Suite:        "cli",
	})

	_, err := suite.Scenarios()
	if !errors.Is(err, ErrNoScenariosForSuite) {
		t.Fatalf("want ErrNoScenariosForSuite, got %v", err)
	}
}

// The undefined-step failure carries a paste-ready definition, with a
// capture group where the step had a quoted run — the difference between
// a reader writing an anchored regexp by hand and pasting one.
func TestUndefinedReportSuggestsAPattern(t *testing.T) {
	t.Parallel()

	report := undefinedReport(
		Scenario{ID: firstScenario, Description: "one"},
		[]Step{{Keyword: KeywordGiven, Text: `the "us-create-happy-path" project tree`}},
	)

	want := "^the \"([^\"]*)\" project tree$"
	if !strings.Contains(report, want) {
		t.Errorf("report does not suggest %q:\n%s", want, report)
	}
}

// A number varies the same way a quoted run does, so it gets a capture
// group too — otherwise every exit code would need its own definition.
func TestSnippetPatternCapturesNumbers(t *testing.T) {
	t.Parallel()

	got := snippetPattern("the command exits with code 1")

	want := `^the command exits with code (\d+)$`
	if got != want {
		t.Errorf("snippetPattern() = %q, want %q", got, want)
	}
}
