package main

import "time"

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
}

// shutdownFloor is the smallest tail worth its own row. Below this the
// engine's last log record and its final turn are the same instant and a
// "0ms shutdown" row would be noise.
const shutdownFloor = 0.0005

// BuildPhases cuts one fixture's wall clock into contiguous slices.
//
// Everything is measured except the leading harness block, which is a
// residual: `go test` reports a subtest's total but never stamps when it
// began, so prep is whatever the other measured slices do not claim.
// That is also why the slices sum to the wall clock by construction —
// the report prints the drift so a hole in this model stays visible.
func BuildPhases(fixture *Fixture) []Phase {
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
			gap := turn.Started.Sub(previousEnd).Seconds()
			if gap < 0 {
				gap = 0
			}

			phases = append(phases, Phase{
				Label: "Engine between turns",
				Detail: "parse the model's result file, write " +
					"artifacts, build the next prompt",
				Kind:     KindDeterministic,
				Owner:    OwnerEngine,
				Seconds:  gap,
				Measured: true,
			})
		}

		phases = append(phases, Phase{
			Label:    "Turn #" + itoa(turn.Number) + " — " + turn.Role,
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

// discoveryBound describes how the test-run slice was measured.
//
// The slice is bounded by log records, so it also carries whatever the
// engine did either side of the subprocess (loading architecture.yaml,
// reaching the checklist loader). When the runner reported its own
// wall clock, saying how much of the slice it accounts for is the
// difference between a span the reader has to trust and one they can
// check.
//
// Only discovery-phase invocations count. A `--fix` run reruns the
// failing test after every applied fix, and those exit records sit in
// the same list — folding them in would claim more measured time than
// the slice contains, which is worse than claiming none.
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
		itoa(count) + " measured runner invocation(s)"
}
