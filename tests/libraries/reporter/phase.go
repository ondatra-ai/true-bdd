package reporter

import (
	"strconv"
	"time"
)

// PhaseKind is what governs a slice's duration.
type PhaseKind string

const (
	// KindDeterministic is Go-or-shell code whose cost is a property of
	// the machine, not of a model's choices.
	KindDeterministic PhaseKind = "deterministic"
	// KindModel is a model deciding how long to take.
	KindModel PhaseKind = "model"
	// KindMixed is a span that measurably contains both and cannot be
	// separated further from the available records.
	KindMixed PhaseKind = "mixed"
)

// PhaseOwner is who runs the code in a slice. Only one of these is code
// true-bdd itself could make faster.
type PhaseOwner string

const (
	// OwnerEngine is true-bdd's own logic.
	OwnerEngine PhaseOwner = "engine"
	// OwnerTests is the framework runner the engine spawns — the
	// project's tests, not the engine.
	OwnerTests PhaseOwner = "tests"
	// OwnerHarness is fixture scaffolding that exists only because this
	// is a test.
	OwnerHarness PhaseOwner = "harness"
)

// Phase is one contiguous slice of a fixture's wall clock.
type Phase struct {
	Label   string
	Detail  string
	Kind    PhaseKind
	Owner   PhaseOwner
	Role    string
	Seconds float64
	// Measured is false for the one slice that is a residual rather
	// than a measurement, so the report can say so in place.
	Measured bool
	// Offset positions the slice on the wall-clock axis for the gantt.
	Offset  float64
	Turn    *Turn
	CostUSD float64
	Tokens  int
	// TestRuns indexes into Fixture.TestRuns for the subprocesses this
	// slice contains. Explicit because a label can't recover it: reruns of
	// the same test share one label, so matching on it would misattribute output.
	TestRuns []int
}

// shutdownFloor is the smallest tail worth its own row. Below this the
// engine's last log record and its final turn are the same instant and a
// "0ms shutdown" row would be noise.
const shutdownFloor = 0.0005

// gapFloor is the smallest engine residual around a test run worth its own
// row; smaller residuals fold into the adjacent slice (see buildPhases).
const gapFloor = 0.0005

// buildPhases cuts one fixture's wall clock into contiguous slices. Every
// slice is measured except the leading harness block — a residual, since
// `go test` never stamps a subtest's start — so slices sum to the wall clock by construction.
func buildPhases(fixture *Fixture) []Phase {
	var phases []Phase

	postSeconds, hasPost := postRunSeconds(fixture)

	phases = appendPrep(phases, fixture, postSeconds)
	phases = appendPreDispatch(phases, fixture)
	phases = appendTurns(phases, fixture)
	phases = appendShutdown(phases, fixture)

	if hasPost {
		phases = append(phases, postRunPhase(fixture, postSeconds))
	}

	offset := 0.0
	for index := range phases {
		phases[index].Offset = offset
		offset += phases[index].Seconds
	}

	return phases
}

// postRunSeconds measures the trailing harness block from the judge's
// own record — the only stamp that exists after the engine exits.
func postRunSeconds(fixture *Fixture) (float64, bool) {
	if fixture.Judge == nil || fixture.Last.IsZero() {
		return 0, false
	}

	seconds := fixture.Judge.At.Sub(fixture.Last).Seconds()
	if seconds < 0 {
		seconds = 0
	}

	return seconds, true
}

// appendPrep adds the leading harness block, the report's one residual.
func appendPrep(phases []Phase, fixture *Fixture, postSeconds float64) []Phase {
	if !fixture.HasWall || fixture.First.IsZero() || fixture.Last.IsZero() {
		return phases
	}

	engineSpan := fixture.Last.Sub(fixture.First).Seconds()

	preSeconds := fixture.Wall.Seconds() - engineSpan - postSeconds
	if preSeconds < 0 {
		preSeconds = 0
	}

	return append(phases, Phase{
		Label: "Fixture prep",
		Detail: "repo-layer copy, input overlay, prep commands " +
			"(npm / playwright install), pre-run snapshot",
		Kind:     KindDeterministic,
		Owner:    OwnerHarness,
		Seconds:  preSeconds,
		Measured: false,
	})
}

// appendPreDispatch adds the engine's work before its first model call:
// the test run, then the checklist load that follows it.
func appendPreDispatch(phases []Phase, fixture *Fixture) []Phase {
	firstDispatch := fixture.Last
	if len(fixture.Turns) > 0 {
		firstDispatch = fixture.Turns[0].Started
	}

	if fixture.First.IsZero() || firstDispatch.IsZero() {
		return phases
	}

	if !fixture.Discovery.Found {
		return append(phases, Phase{
			Label: "Engine start-up",
			Detail: "architecture load, test discovery, checklist " +
				"parse, template render",
			Kind:     KindDeterministic,
			Owner:    OwnerEngine,
			Seconds:  firstDispatch.Sub(fixture.First).Seconds(),
			Measured: true,
		})
	}

	detail := "engine spawned the " + fixture.Discovery.Framework +
		" runner to find failing tests · " + discoveryBound(fixture)
	if fixture.Discovery.Outcome != "" {
		detail += " · " + fixture.Discovery.Outcome
	}

	phases = append(phases, Phase{
		Label:    "Test run (" + fixture.Discovery.Framework + ")",
		Detail:   detail,
		Kind:     KindDeterministic,
		Owner:    OwnerTests,
		Seconds:  fixture.Discovery.End.Sub(fixture.First).Seconds(),
		Measured: true,
		// The opening slice is bounded by log records, so it contains
		// every discovery-phase invocation rather than exactly one.
		TestRuns: discoveryRuns(fixture),
	})

	return append(phases, Phase{
		Label: "Checklist load + prompt render",
		Detail: "checklist parse, template render, prompt " +
			"artifacts written to tmp/",
		Kind:     KindDeterministic,
		Owner:    OwnerEngine,
		Seconds:  firstDispatch.Sub(fixture.Discovery.End).Seconds(),
		Measured: true,
	})
}

// appendTurns adds each model turn, with the engine's own work between.
func appendTurns(phases []Phase, fixture *Fixture) []Phase {
	var previousEnd time.Time

	for _, turn := range fixture.Turns {
		if !previousEnd.IsZero() && !turn.Started.IsZero() {
			phases = appendGap(phases, fixture, previousEnd, turn.Started)
		}

		phases = append(phases, Phase{
			Label:    "Turn #" + strconv.Itoa(turn.Number) + " — " + turn.Role,
			Detail:   turn.CLI + " · " + turn.Model,
			Kind:     KindModel,
			Role:     turn.Role,
			Seconds:  turn.Seconds(),
			Measured: true,
			Turn:     turn,
		})

		previousEnd = turn.Ended
	}

	return phases
}

// appendGap cuts the span between two model turns into what actually
// happened: a PostFix rerun (up to a docker-build-and-run suite) gets its
// own slice instead of folding into "Engine between turns", hiding a `--fix` run's only validating step.
func appendGap(phases []Phase, fixture *Fixture, from, until time.Time) []Phase {
	runs := testRunsWithin(fixture, from, until)
	if len(runs) == 0 {
		return append(phases, engineGapPhase(spanSeconds(from, until)))
	}

	cursor := from

	for _, position := range runs {
		run := fixture.TestRuns[position]

		start := run.Started()
		if start.Before(cursor) {
			start = cursor
		}

		lead := spanSeconds(cursor, start)
		if lead > gapFloor {
			phases = append(phases, engineGapPhase(lead))
			cursor = start
		}

		phases = append(phases, testRunPhase(run, position, spanSeconds(cursor, run.At)))
		cursor = run.At
	}

	trail := spanSeconds(cursor, until)
	if trail > gapFloor {
		return append(phases, engineGapPhase(trail))
	}

	phases[len(phases)-1].Seconds += trail

	return phases
}

// testRunsWithin lists the runner subprocesses inside the window, oldest
// first, as indices into fixture.TestRuns. A run with no exit timestamp is
// left unplaceable and excluded rather than mispositioned.
func testRunsWithin(fixture *Fixture, from, until time.Time) []int {
	var runs []int

	for position, run := range fixture.TestRuns {
		if run.At.IsZero() || !run.At.After(from) || run.At.After(until) {
			continue
		}

		runs = append(runs, position)
	}

	return runs
}

// discoveryRuns indexes the discovery-phase invocations, which are what
// the opening test-run slice spans.
func discoveryRuns(fixture *Fixture) []int {
	var runs []int

	for position, run := range fixture.TestRuns {
		if run.IsDiscovery() {
			runs = append(runs, position)
		}
	}

	return runs
}

// engineGapPhase is the engine's own work around a turn boundary.
func engineGapPhase(seconds float64) Phase {
	return Phase{
		Label: "Engine between turns",
		Detail: "parse the model's result file, write artifacts, " +
			"build the next prompt",
		Kind:     KindDeterministic,
		Owner:    OwnerEngine,
		Seconds:  seconds,
		Measured: true,
	}
}

// testRunPhase is one runner subprocess on the timeline: bounded by the
// surrounding turn records, so it can carry a sliver of engine work either
// side, with the detail stating the runner's own measured wall clock.
func testRunPhase(run TestRun, position int, seconds float64) Phase {
	return Phase{
		Label:    "Test run (" + run.Label() + ")",
		Detail:   testRunDetail(run),
		Kind:     KindDeterministic,
		Owner:    OwnerTests,
		Seconds:  seconds,
		Measured: true,
		TestRuns: []int{position},
	}
}

// testRunDetail says why the engine spawned the runner and how it ended.
func testRunDetail(run TestRun) string {
	detail := "engine re-ran the test to decide whether the fix worked"
	if run.Phase == phaseDiscover {
		detail = "engine spawned the runner to find failing tests"
	}

	if run.Duration > 0 {
		detail += " · " + formatSeconds(run.Duration.Seconds()) + " measured"
	}

	switch {
	case run.Error != "":
		detail += " · never started: " + run.Error
	case !run.HasExit:
		detail += " · no exit status recorded"
	case run.ExitCode == 0:
		detail += " · exit 0, the fix holds"
	default:
		detail += " · exit " + strconv.Itoa(run.ExitCode) + ", still failing"
	}

	return detail
}

// spanSeconds is a non-negative wall-clock span.
func spanSeconds(from, until time.Time) float64 {
	seconds := until.Sub(from).Seconds()
	if seconds < 0 {
		return 0
	}

	return seconds
}

// appendShutdown adds the engine's work after its last turn.
func appendShutdown(phases []Phase, fixture *Fixture) []Phase {
	var previousEnd time.Time
	if len(fixture.Turns) > 0 {
		previousEnd = fixture.Turns[len(fixture.Turns)-1].Ended
	}

	if previousEnd.IsZero() || fixture.Last.IsZero() {
		return phases
	}

	tail := fixture.Last.Sub(previousEnd).Seconds()
	if tail <= shutdownFloor {
		return phases
	}

	return append(phases, Phase{
		Label:    "Engine shutdown",
		Detail:   "final verdict, error wrapping, log flush",
		Kind:     KindDeterministic,
		Owner:    OwnerEngine,
		Seconds:  tail,
		Measured: true,
	})
}

// postRunPhase is the trailing harness block. Its span is measured but
// its contents are not separable: the snapshot, the diff, teardown and
// the judge's model call all land inside it.
func postRunPhase(fixture *Fixture, seconds float64) Phase {
	return Phase{
		Label: "Post-run + judge",
		Detail: "post-run snapshot and diff, fixture teardown " +
			"(docker compose down), then the harness judge " +
			"call on claude · sonnet",
		Kind:     KindMixed,
		Owner:    OwnerHarness,
		Role:     roleJudge,
		Seconds:  seconds,
		Measured: true,
		CostUSD:  fixture.Judge.CostUSD,
		Tokens:   fixture.Judge.Tokens,
	}
}

// discoveryBound describes how the test-run slice was measured, counting
// only discovery-phase invocations: a `--fix` run's rerun records share the
// list, and folding them in would claim more measured time than the slice contains.
func discoveryBound(fixture *Fixture) string {
	measured := time.Duration(0)
	count := 0

	for _, run := range fixture.TestRuns {
		if !run.IsDiscovery() {
			continue
		}

		measured += run.Duration
		count++
	}

	if measured == 0 {
		return "bounded by the next engine log record"
	}

	return "bounded by the next engine log record, of which " +
		formatSeconds(measured.Seconds()) + " is " +
		strconv.Itoa(count) + " measured runner invocation(s)"
}
