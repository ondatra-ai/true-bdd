package bddgo

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// ErrNoScenariosForSuite signals that the registry has scenarios but
// none the named suite owns. A suite that runs nothing passes trivially,
// so it is refused: the two ways to reach this state — a scenario whose
// `service:` is misspelled, and a suite wired to a service nobody writes
// scenarios for — are both bugs, and both look exactly like success.
var ErrNoScenariosForSuite = errors.New("no registry scenario names this suite's service")

// ErrAmbiguousStep signals a step matched by more than one definition.
// Refused rather than resolved by registration order: which one runs
// would then depend on the order of two lines in a file nobody reads
// while writing scenarios.
var ErrAmbiguousStep = errors.New("step matches more than one definition")

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
	// SubtestName renders a scenario's Go subtest name. Optional; the
	// default is the scenario id. A suite whose scenarios each drive a
	// named on-disk fixture overrides it so `-run` keeps naming what
	// people already type.
	SubtestName func(Scenario) string
}

// stepDef is one registered binding: a pattern and the code it runs.
type stepDef[S any] struct {
	pattern *regexp.Regexp
	run     func(*S, []string) error
}

// Suite is a registry-driven test suite. S is the per-scenario state the
// suite's own step definitions share — the tmpdir, the run result, the
// browser page. bddgo never looks inside it; it only builds one per
// scenario and hands it to each step in turn.
type Suite[S any] struct {
	t     *testing.T
	opts  Options
	spec  SuiteSpec
	steps []stepDef[S]
	// newState builds the per-scenario state. Registered through Init.
	newState func(*World) (*S, error)
}

// New loads the architectural spec, resolves the named suite and returns
// an empty suite ready for step registration. A spec that does not
// declare the suite fails here, before any scenario is read.
func New[S any](t *testing.T, opts Options) *Suite[S] {
	t.Helper()

	spec, err := LoadSuiteSpec(opts.Architecture, opts.Suite)
	if err != nil {
		t.Fatalf("bddgo: %v", err)
	}

	return &Suite[S]{t: t, opts: opts, spec: spec}
}

// Spec returns the suite's entry from the architectural spec.
func (s *Suite[S]) Spec() SuiteSpec {
	return s.spec
}

// Init registers the per-scenario state constructor. Required.
//
// The constructor receives the World — the scenario and its *testing.T —
// so a suite that needs teardown registers it with w.T.Cleanup and gets
// LIFO ordering against everything the steps register later.
func (s *Suite[S]) Init(newState func(*World) (*S, error)) {
	s.newState = newState
}

// Step binds a regexp to the code that executes a matching step. The
// pattern's capture groups become the step's arguments, in order.
//
// Anchoring is the caller's business — `^…$` is the usual and the safe
// choice, since an unanchored pattern quietly matches steps it was never
// meant to.
func (s *Suite[S]) Step(pattern string, run func(*S, []string) error) {
	s.t.Helper()

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		s.t.Fatalf("bddgo: step pattern %q: %v", pattern, err)
	}

	s.steps = append(s.steps, stepDef[S]{pattern: compiled, run: run})
}

// Scenarios returns the registry scenarios this suite owns, in id order.
func (s *Suite[S]) Scenarios() ([]Scenario, error) {
	all, err := LoadRegistry(s.opts.Registry)
	if err != nil {
		return nil, err
	}

	owned := make([]Scenario, 0, len(all))

	for _, scenario := range all {
		if s.spec.Owns(scenario) {
			owned = append(owned, scenario)
		}
	}

	if len(owned) == 0 {
		return nil, fmt.Errorf("%s: suite %q (service %q): %w",
			s.opts.Registry, s.spec.Name, s.spec.Service, ErrNoScenariosForSuite)
	}

	return owned, nil
}

// resolved is one scenario step paired with the definition that will run
// it and the arguments its pattern captured.
type resolved[S any] struct {
	step Step
	def  *stepDef[S]
	args []string
}

// resolve binds every step of a scenario to a definition WITHOUT running
// any of them, and returns the ones it could not bind.
//
// A pre-pass rather than a match-as-you-go loop, for the same reason the
// engine validates a whole spec before spawning anything: a scenario
// whose fourth step is undefined should say so before its first step
// creates a tmpdir and spawns a subprocess. It is also what lets a
// failure list EVERY missing definition at once, which is what makes the
// fix a single edit instead of four rounds of discovery.
func (s *Suite[S]) resolve(scenario Scenario) ([]resolved[S], []Step, error) {
	bound := make([]resolved[S], 0, len(scenario.Steps))

	var undefined []Step

	for _, step := range scenario.Steps {
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
			return nil, nil, fmt.Errorf("%s: %q: %w (%s)",
				scenario.ID, step.Text, ErrAmbiguousStep, patternList(matches))
		}
	}

	return bound, undefined, nil
}

func patternList[S any](matches []resolved[S]) string {
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match.def.pattern.String())
	}

	return strings.Join(names, " | ")
}

// Undefined resolves every owned scenario without running anything and
// returns the steps no definition matches, keyed by scenario id. Empty
// means the suite covers its half of the registry.
//
// Exported because that question — "is every written-down behaviour
// executable?" — is exactly what `build tests` asks, and answering it by
// running the suite would cost minutes and a subprocess per scenario.
func (s *Suite[S]) Undefined() (map[string][]Step, error) {
	scenarios, err := s.Scenarios()
	if err != nil {
		return nil, err
	}

	gaps := map[string][]Step{}

	for _, scenario := range scenarios {
		_, undefined, resolveErr := s.resolve(scenario)
		if resolveErr != nil {
			return nil, resolveErr
		}

		if len(undefined) > 0 {
			gaps[scenario.ID] = undefined
		}
	}

	return gaps, nil
}

// Run executes every scenario the suite owns as a subtest.
func (s *Suite[S]) Run() {
	s.t.Helper()

	if s.newState == nil {
		s.t.Fatal("bddgo: Init was never called — a suite with no state constructor cannot run a scenario")
	}

	scenarios, err := s.Scenarios()
	if err != nil {
		s.t.Fatalf("bddgo: %v", err)
	}

	for _, scenario := range scenarios {
		s.t.Run(s.subtestName(scenario), func(t *testing.T) {
			s.runScenario(t, scenario)
		})
	}
}

// RunScenarios executes a caller-supplied scenario list instead of the
// whole owned set. Used by a suite that has to pair each scenario with
// on-disk data and wants to check that pairing first.
func (s *Suite[S]) RunScenarios(scenarios []Scenario) {
	s.t.Helper()

	for _, scenario := range scenarios {
		s.t.Run(s.subtestName(scenario), func(t *testing.T) {
			s.runScenario(t, scenario)
		})
	}
}

func (s *Suite[S]) subtestName(scenario Scenario) string {
	if s.opts.SubtestName == nil {
		return scenario.ID
	}

	return s.opts.SubtestName(scenario)
}

// runScenario resolves, builds state, and executes the steps in order.
func (s *Suite[S]) runScenario(t *testing.T, scenario Scenario) {
	t.Helper()

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

	for index, step := range bound {
		err = step.def.run(state, step.args)
		if err != nil {
			// The remaining steps are reported rather than run: once a
			// When has not happened, every Then after it would assert
			// against a state the scenario never reached, and their
			// failures would bury the one that matters.
			t.Fatalf("%s\n  step %d/%d failed: %s\n  %v\n  not run: %s",
				scenario.ID, index+1, len(bound), step.step, err, remaining(bound[index+1:]))
		}
	}
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
