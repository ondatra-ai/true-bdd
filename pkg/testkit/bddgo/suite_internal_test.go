package bddgo

import (
	"errors"
	"strings"
	"testing"
)

// firstScenario is the id these tests expect selection to return.
const firstScenario = "E2E-001"

// suiteName is the one suite specDoc declares.
const suiteName = "bdd-cli"

// The step texts registryDoc and modelRegistryDoc use, named so the two
// halves of a cross-check assertion cannot drift apart in a test.
const (
	stepATree     = "a tree"
	stepCLIRuns   = "the CLI runs"
	stepItWorked  = "it worked"
	clauseWording = "the wording survived"
)

// specDoc is a one-suite architectural spec: the cli suite, exercising
// the bdd-cli service.
const specDoc = `
architecture:
  testing:
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

	suite, err := New[testState](Options{
		Registry:     writeDoc(t, "scenarios.yaml", registryDoc),
		Architecture: writeDoc(t, "architecture.yaml", specDoc),
		Suite:        suiteName,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	suite.Init(func(*World) (*testState, error) { return &testState{}, nil })

	return suite
}

// A suite runs the scenarios its own service owns and nobody else's —
// the join is one line of the architectural spec, not a constant in the
// test binary.
func TestSuiteSelectsByService(t *testing.T) {
	t.Parallel()

	scenarios, err := newSuite(t).Owned()
	if err != nil {
		t.Fatalf("Owned: %v", err)
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

	_, err := LoadSuiteSpec(writeDoc(t, "architecture.yaml", specDoc), "bdd-clii")
	if !errors.Is(err, ErrSuiteNotDeclared) {
		t.Fatalf("want ErrSuiteNotDeclared, got %v", err)
	}

	if !strings.Contains(err.Error(), "declared: bdd-cli") {
		t.Errorf("the refusal must name what IS declared, got %q", err)
	}
}

// Every step of every owned scenario must bind, and Unbound answers
// that without running a thing — which is what makes it affordable for
// `build tests` to ask on every walk.
func TestUnboundListsEveryUnboundStep(t *testing.T) {
	t.Parallel()

	suite := newSuite(t)
	suite.Step(`^a tree$`, func(*testState, []string) error { return nil })

	gaps, err := suite.Unbound()
	if err != nil {
		t.Fatalf("Unbound: %v", err)
	}

	steps := gaps[firstScenario]
	if len(steps) != 1 || steps[0].Text != stepCLIRuns {
		t.Fatalf("want the unbound When step, got %+v", gaps)
	}
}

// With every step bound there is no gap, and the report says so by
// being empty rather than by omitting the scenario.
func TestUnboundIsEmptyWhenEveryStepBinds(t *testing.T) {
	t.Parallel()

	suite := newSuite(t)
	suite.Step(`^a tree$`, func(*testState, []string) error { return nil })
	suite.Step(`^the CLI runs$`, func(*testState, []string) error { return nil })

	gaps, err := suite.Unbound()
	if err != nil {
		t.Fatalf("Unbound: %v", err)
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

	_, err := suite.Unbound()
	if !errors.Is(err, ErrAmbiguousStep) {
		t.Fatalf("want ErrAmbiguousStep, got %v", err)
	}
}

// Selection itself is not where an empty suite is refused: Owned simply
// reports what the registry assigns, and whether owning nothing is a bug
// is a question about the repository that CheckCoverage asks once.
func TestOwnedIsEmptyWhenNoScenarioNamesTheSuite(t *testing.T) {
	t.Parallel()

	suite, err := New[testState](Options{
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
		Suite:        suiteName,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	owned, err := suite.Owned()
	if err != nil {
		t.Fatalf("Owned: %v", err)
	}

	if len(owned) != 0 {
		t.Fatalf("want no owned scenarios, got %+v", owned)
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

// modelRegistryDoc drives the model-run paths: one act, one deterministic
// assertion, two clauses.
const modelRegistryDoc = `
scenarios:
  E2E-001:
    description: mine
    service: bdd-cli
    user_stories: []
    merged_steps:
      when:
        - the CLI runs
        - and: 'llm: do the thing'
      then:
        - it worked
        - and: 'judge: the wording survived'
        - but: 'judge: nothing else moved'
`

// modelState records what the two model-run paths were handed, so a test
// can assert the batching rather than the individual calls.
type modelState struct {
	acted   []string
	clauses []string
}

func (m *modelState) Act(step Step) error {
	m.acted = append(m.acted, step.Text)

	return nil
}

func (m *modelState) Judge(clauses []Step) error {
	for _, clause := range clauses {
		m.clauses = append(m.clauses, clause.Text)
	}

	return nil
}

func newModelSuite(t *testing.T, state *modelState) *Suite[modelState] {
	t.Helper()

	suite, err := New[modelState](Options{
		Registry:     writeDoc(t, "scenarios.yaml", modelRegistryDoc),
		Architecture: writeDoc(t, "architecture.yaml", specDoc),
		Suite:        suiteName,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	suite.Init(func(*World) (*modelState, error) { return state, nil })
	suite.Step(`^the CLI runs$`, func(*modelState, []string) error { return nil })
	suite.Step(`^it worked$`, func(*modelState, []string) error { return nil })

	return suite
}

// A model-run step needs no definition and must never be reported as a
// gap — otherwise `build tests --fix` would be sent to author a pattern
// for a clause written precisely because no pattern could settle it.
func TestUnboundIgnoresModelRunSteps(t *testing.T) {
	t.Parallel()

	gaps, err := newModelSuite(t, &modelState{}).Unbound()
	if err != nil {
		t.Fatalf("Unbound: %v", err)
	}

	if len(gaps) != 0 {
		t.Fatalf("want no gaps, got %+v", gaps)
	}
}

// Acts run in place and clauses are batched: one Judge call carrying
// every clause in registry order, taken after the last step passed.
func TestRunScenarioActsInPlaceAndBatchesClauses(t *testing.T) {
	t.Parallel()

	state := &modelState{}
	suite := newModelSuite(t, state)

	run := suite.Scenario(t, firstScenario)
	run.When(stepCLIRuns)
	run.And("llm: do the thing")
	run.Then(stepItWorked)
	run.And("judge: " + clauseWording)
	run.But("judge: nothing else moved")
	run.Done()

	if len(state.acted) != 1 || state.acted[0] != "do the thing" {
		t.Errorf("want one act, got %+v", state.acted)
	}

	want := []string{clauseWording, "nothing else moved"}
	if strings.Join(state.clauses, "|") != strings.Join(want, "|") {
		t.Errorf("want clauses %v in order, got %v", want, state.clauses)
	}
}

// A clause nobody grades still READS as coverage, which is why the
// missing interface is a refusal rather than a skip.
func TestJudgeClausesRefusesStateThatCannotGrade(t *testing.T) {
	t.Parallel()

	err := judgeClauses(&testState{},
		[]Step{{Keyword: KeywordThen, Mode: ModeRule, Text: clauseWording}})
	if !errors.Is(err, ErrNoJudge) {
		t.Fatalf("want ErrNoJudge, got %v", err)
	}

	if !strings.Contains(err.Error(), clauseWording) {
		t.Errorf("the refusal must quote a clause, got %q", err)
	}
}

// A scenario with no clauses never reaches a judge at all — which is the
// whole point of moving what a comparison can settle into definitions.
func TestJudgeClausesSkipsWhenNoClauses(t *testing.T) {
	t.Parallel()

	err := judgeClauses(&testState{}, nil)
	if err != nil {
		t.Fatalf("want no judge call, got %v", err)
	}
}
