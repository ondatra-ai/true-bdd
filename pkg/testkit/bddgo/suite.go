package bddgo

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// ErrNoScenariosForSuite signals that the registry has scenarios but
// none the named suite owns — refused because a suite that runs nothing
// passes trivially, indistinguishable from a bug that looks like success.
var ErrNoScenariosForSuite = errors.New("no registry scenario names this suite's service")

// ErrAmbiguousStep signals a step matched by more than one definition.
// Refused rather than resolved by registration order, which would make
// behavior depend on line order in a file nobody reads for that.
var ErrAmbiguousStep = errors.New("step matches more than one definition")

// AmbiguousStepError is the ambiguity resolve found, with its three
// parts kept apart so a caller can render them itself.
type AmbiguousStepError struct {
	Scenario string
	Step     string
	Patterns []string
}

func (e *AmbiguousStepError) Error() string {
	return fmt.Sprintf("%s: %q: %v (%s)",
		e.Scenario, e.Step, ErrAmbiguousStep, strings.Join(e.Patterns, " | "))
}

// Unwrap lets errors.Is reach the sentinel, which is what tells a caller
// this is a finding about a step rather than the suite failing to answer.
func (e *AmbiguousStepError) Unwrap() error { return ErrAmbiguousStep }

// Options configures one suite run.
type Options struct {
	// Registry is the scenario registry's path — `documents.scenarios_yaml`
	// from the host's true-bdd.yaml, conventionally docs/scenarios.yaml.
	Registry string
	// Architecture is the architectural spec's path.
	Architecture string
	// Suite names the `architecture.testing.suites[]` entry this binary
	// runs. Its `service:` selects the scenarios.
	Suite string
}

// stepDef is one registered binding: a pattern and the code it runs.
type stepDef[S any] struct {
	pattern *regexp.Regexp
	run     func(*S, []string) error
}

// Suite is a registry-driven test suite. S is the per-scenario state the
// suite's own step definitions share — the tmpdir, the run result, the
// browser page — which bddgo builds once per scenario but never inspects.
type Suite[S any] struct {
	opts  Options
	spec  SuiteSpec
	steps []stepDef[S]
	// err holds the first step pattern that would not compile. Recorded
	// rather than raised, since Step is called from TestMain where there
	// is no *testing.T to fail; each scenario reports it instead.
	err error
	// newState builds the per-scenario state. Registered through Init.
	newState func(*World) (*S, error)

	// The registry is read once per process. With one top-level Go test
	// per scenario, a load inside each of them would parse the same
	// document two hundred times.
	loadOnce sync.Once
	loaded   []Scenario
	loadErr  error
}

// New loads the architectural spec, resolves the named suite and returns
// an empty suite ready for step registration. It takes no *testing.T:
// the suite is built once per process, in TestMain, and outlives every test.
func New[S any](opts Options) (*Suite[S], error) {
	spec, err := LoadSuiteSpec(opts.Architecture, opts.Suite)
	if err != nil {
		return nil, fmt.Errorf("bddgo: %w", err)
	}

	return &Suite[S]{opts: opts, spec: spec}, nil
}

// Spec returns the suite's entry from the architectural spec.
func (s *Suite[S]) Spec() SuiteSpec {
	return s.spec
}

// Init registers the per-scenario state constructor. Required. The
// constructor receives the World (scenario + *testing.T), so teardown
// registered via w.T.Cleanup gets LIFO ordering against later steps.
func (s *Suite[S]) Init(newState func(*World) (*S, error)) {
	s.newState = newState
}

// Step binds a regexp to the code that executes a matching step. The
// pattern's capture groups become the step's arguments, in order.
// Anchoring (`^…$`) is the caller's business — unanchored quietly over-matches.
func (s *Suite[S]) Step(pattern string, run func(*S, []string) error) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		if s.err == nil {
			s.err = fmt.Errorf("bddgo: step pattern %q: %w", pattern, err)
		}

		return
	}

	s.steps = append(s.steps, stepDef[S]{pattern: compiled, run: run})
}

// all returns every scenario in the registry, parsed once per process.
func (s *Suite[S]) all() ([]Scenario, error) {
	s.loadOnce.Do(func() {
		s.loaded, s.loadErr = LoadRegistry(s.opts.Registry)
	})

	return s.loaded, s.loadErr
}

// Owned returns the registry scenarios this suite owns, in id order. An
// empty result is not an error here — whether that's a bug is
// CheckCoverage's question to ask (see ErrNoScenariosForSuite).
func (s *Suite[S]) Owned() ([]Scenario, error) {
	all, err := s.all()
	if err != nil {
		return nil, err
	}

	owned := make([]Scenario, 0, len(all))

	for _, scenario := range all {
		if s.spec.Owns(scenario) {
			owned = append(owned, scenario)
		}
	}

	return owned, nil
}

// Lookup returns the registry scenario with the given id, whether or not
// this suite owns it. The second result reports whether it was found.
func (s *Suite[S]) Lookup(scenarioID string) (Scenario, bool, error) {
	all, err := s.all()
	if err != nil {
		return Scenario{}, false, err
	}

	for _, scenario := range all {
		if scenario.ID == scenarioID {
			return scenario, true, nil
		}
	}

	return Scenario{}, false, nil
}

// resolved is one scenario step paired with the definition that will run
// it and the arguments its pattern captured. def is nil for a model-run
// step, which binds to no pattern by construction.
type resolved[S any] struct {
	step Step
	def  *stepDef[S]
	args []string
}

// resolve binds every step of a scenario to a definition WITHOUT running
// any of them, and returns the ones it could not bind — a pre-pass so a
// scenario's fourth step can fail before the first spawns a subprocess.
func (s *Suite[S]) resolve(scenario Scenario) ([]resolved[S], []Step, error) {
	bound := make([]resolved[S], 0, len(scenario.Steps))

	var undefined []Step

	for _, step := range scenario.Steps {
		// A model-run step is bound the moment it is read: what it asks
		// for is precisely what no regexp can settle, so the pattern
		// table would only ever report it undefined.
		if step.Mode != ModeDeterministic {
			bound = append(bound, resolved[S]{step: step})

			continue
		}

		matches := make([]resolved[S], 0, 1)

		for index := range s.steps {
			def := &s.steps[index]

			args := def.pattern.FindStringSubmatch(step.Text)
			if args == nil {
				continue
			}

			matches = append(matches, resolved[S]{step: step, def: def, args: args[1:]})
		}

		switch len(matches) {
		case 0:
			undefined = append(undefined, step)
		case 1:
			bound = append(bound, matches[0])
		default:
			return nil, nil, &AmbiguousStepError{
				Scenario: scenario.ID,
				Step:     step.Text,
				Patterns: patternList(matches),
			}
		}
	}

	return bound, undefined, nil
}

func patternList[S any](matches []resolved[S]) []string {
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match.def.pattern.String())
	}

	return names
}

// runScenario resolves, builds state, and executes the steps in order.
// Reached only through Run.Done — there is no entry point that walks
// the owned set directly; an unrun scenario is a gap CheckCoverage names.
func (s *Suite[S]) runScenario(t *testing.T, scenario Scenario) {
	t.Helper()

	if s.newState == nil {
		t.Fatal("bddgo: Init was never called — a suite with no state constructor cannot run a scenario")
	}

	bound, undefined, err := s.resolve(scenario)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if len(undefined) > 0 {
		t.Fatalf("%s", undefinedReport(scenario, undefined))
	}

	world := &World{T: t, Scenario: scenario, Suite: s.spec}

	state, err := s.newState(world)
	if err != nil {
		t.Fatalf("%s: building scenario state: %v", scenario.ID, err)
	}

	var clauses []Step

	for index, step := range bound {
		err = s.runStep(state, step, &clauses)
		if err != nil {
			// The remaining steps are reported rather than run: once a When
			// has not happened, every Then after it would assert against a
			// state the scenario never reached, burying the failure that matters.
			t.Fatalf("%s\n  step %d/%d failed: %s\n  %v\n  not run: %s",
				scenario.ID, index+1, len(bound), step.step, err, remaining(bound[index+1:]))
		}
	}

	// The verdict comes last and only here, so a scenario whose exit code
	// was already wrong stops at the t.Fatalf above — paying a model to
	// read the wreckage would answer a question nobody asked.
	err = judgeClauses(state, clauses)
	if err != nil {
		t.Fatalf("%s\n  %v", scenario.ID, err)
	}
}

// runStep dispatches one bound step to whoever executes it.
func (s *Suite[S]) runStep(state *S, step resolved[S], clauses *[]Step) error {
	switch step.step.Mode {
	case ModeRule:
		*clauses = append(*clauses, step.step)

		return nil

	case ModeAct:
		actor, ok := any(state).(Actor)
		if !ok {
			return fmt.Errorf("%w: %s", ErrNoActor, step.step)
		}

		err := actor.Act(step.step)
		if err != nil {
			return fmt.Errorf("%s: %w", step.step, err)
		}

		return nil

	case ModeDeterministic:
		return step.def.run(state, step.args)

	default:
		return fmt.Errorf("%w: %s", ErrMalformedStep, step.step)
	}
}

// judgeClauses hands a scenario's collected clauses to its state. A
// scenario with none never reaches a judge at all, which is the whole
// point of moving what a comparison can settle into step definitions.
func judgeClauses[S any](state *S, clauses []Step) error {
	if len(clauses) == 0 {
		return nil
	}

	judge, ok := any(state).(Judgeable)
	if !ok {
		return fmt.Errorf("%w (%d clause(s), first: %s)",
			ErrNoJudge, len(clauses), clauses[0])
	}

	// Wrapped by role, not by scenario id: runScenario already heads the
	// failure with the id and a suite's own failures carry it too, so
	// repeating it here printed it three times in one message.
	err := judge.Judge(clauses)
	if err != nil {
		return fmt.Errorf("judging %d clause(s): %w", len(clauses), err)
	}

	return nil
}

func remaining[S any](rest []resolved[S]) string {
	if len(rest) == 0 {
		return "(none)"
	}

	texts := make([]string, 0, len(rest))
	for _, item := range rest {
		texts = append(texts, item.step.String())
	}

	return strings.Join(texts, "; ")
}
