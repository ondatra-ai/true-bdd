package main

import (
	"math"
	"strings"
	"testing"
	"time"
)

// driftTolerance is how far the accounted timeline may land from the
// wall clock before the phase model is considered to have a hole. The
// slices are built from millisecond-resolution stamps, so a few
// milliseconds of rounding is expected; anything larger is a real gap.
const driftTolerance = 0.01

// modelOpus is the stand-in model name for the synthetic turns.
const modelOpus = "opus"

// at builds a timestamp offset from a fixed base, so the tests read as
// "this many seconds into the run".
func at(base time.Time, seconds float64) time.Time {
	return base.Add(time.Duration(seconds * float64(time.Second)))
}

// buildTestFixture assembles a fixture shaped like a real run: engine
// start, a discovery span, three turns, then the harness's judge call.
func buildTestFixture() *Fixture {
	base := time.Date(2026, time.August, 9, 20, 21, 12, 0, time.UTC)

	turns := []*Turn{
		{
			Number: 1, Role: rolePrompt, CLI: cliClaude, Model: modelOpus,
			Started: at(base, 1.046), Ended: at(base, 11.066),
			Duration: 10020 * time.Millisecond, Status: TurnOK,
		},
		{
			Number: 2, Role: roleFix, CLI: cliClaude, Model: modelOpus,
			Started: at(base, 11.067), Ended: at(base, 133.692),
			Duration: 122623 * time.Millisecond, Status: TurnOK,
		},
		{
			Number: 3, Role: roleApply, CLI: cliCrush, Model: "glm",
			Started: at(base, 133.693), Ended: at(base, 198.137),
			Duration: 64443 * time.Millisecond, Status: TurnOK,
		},
	}

	fixture := &Fixture{
		Name:    "example",
		Turns:   turns,
		First:   base,
		Last:    at(base, 198.137),
		Wall:    time.Duration(205.88 * float64(time.Second)),
		HasWall: true,
		Judge: &JudgeCall{
			At: at(base, 202.4), CostUSD: 0.4174, Tokens: 69417,
		},
		Discovery: Discovery{
			Framework: frameworkPlaywright, End: at(base, 1.033), Found: true,
		},
	}

	fixture.Phases = BuildPhases(fixture)
	for _, phase := range fixture.Phases {
		fixture.PhaseTotal += phase.Seconds
	}

	return fixture
}

// TestPhasesSumToWallClock is the report's central invariant: the
// timeline accounts for the whole run, so no time can hide in an
// unlabelled remainder.
func TestPhasesSumToWallClock(t *testing.T) {
	fixture := buildTestFixture()

	drift := math.Abs(fixture.Wall.Seconds() - fixture.PhaseTotal)
	if drift > driftTolerance {
		t.Errorf("phases sum to %.3fs but wall clock is %.3fs (drift %.3fs)",
			fixture.PhaseTotal, fixture.Wall.Seconds(), drift)
	}
}

// TestPhasesAreContiguous checks that each slice starts where the
// previous one ended. A gap or an overlap would make the gantt lie about
// where in the run a slice happened.
func TestPhasesAreContiguous(t *testing.T) {
	fixture := buildTestFixture()

	expected := 0.0

	for _, phase := range fixture.Phases {
		if math.Abs(phase.Offset-expected) > driftTolerance {
			t.Errorf("phase %q starts at %.3fs, expected %.3fs",
				phase.Label, phase.Offset, expected)
		}

		expected += phase.Seconds
	}
}

// TestPhaseOwnership checks that the test subprocess is not counted as
// engine time. Conflating them was the thing the Go-side breakdown
// exists to prevent: it would report a slow project test suite as slow
// engine code.
func TestPhaseOwnership(t *testing.T) {
	fixture := buildTestFixture()

	byOwner := map[PhaseOwner]float64{}
	for _, phase := range fixture.Phases {
		byOwner[phase.Owner] += phase.Seconds
	}

	if got := byOwner[OwnerTests]; math.Abs(got-1.033) > driftTolerance {
		t.Errorf("test subprocess = %.3fs, want 1.033s", got)
	}

	// Engine logic is the checklist load plus the two inter-turn gaps —
	// milliseconds, and emphatically not the discovery span.
	if got := byOwner[OwnerEngine]; got > 0.1 {
		t.Errorf("engine logic = %.3fs, want milliseconds — is the test run "+
			"being counted as engine time?", got)
	}
}

// TestModelTimeExcludesDeterministicWork checks the headline split: the
// three turns are non-deterministic, everything else is not.
func TestModelTimeExcludesDeterministicWork(t *testing.T) {
	fixture := buildTestFixture()

	var model, deterministic float64

	for _, phase := range fixture.Phases {
		switch phase.Kind {
		case KindModel:
			model += phase.Seconds
		case KindDeterministic:
			deterministic += phase.Seconds
		case KindMixed:
		}
	}

	wantModel := 10.020 + 122.623 + 64.443
	if math.Abs(model-wantModel) > driftTolerance {
		t.Errorf("model time = %.3fs, want %.3fs", model, wantModel)
	}

	if deterministic <= 0 {
		t.Error("deterministic time is zero — prep and discovery went missing")
	}
}

// testRunPhaseDetail returns the detail line of the timeline's test-run
// slice — the one sentence that tells the reader how much of the slice
// is actually measured.
func testRunPhaseDetail(t *testing.T, fixture *Fixture) string {
	t.Helper()

	for _, phase := range BuildPhases(fixture) {
		if phase.Owner == OwnerTests {
			return phase.Detail
		}
	}

	t.Fatal("no test-run phase in the timeline")

	return ""
}

// TestDiscoveryBoundCountsOnlyDiscoveryRuns pins the discovery slice's
// measured-time claim to the runs that happened inside it. A `--fix`
// run reruns the failing test after each fix, and those exit records
// live in the same list — counting them would attribute fix-loop time
// to a slice that closed before the first turn.
func TestDiscoveryBoundCountsOnlyDiscoveryRuns(t *testing.T) {
	t.Parallel()

	fixture := buildTestFixture()
	fixture.TestRuns = []TestRun{
		{Framework: frameworkPlaywright, Phase: phaseDiscover, Duration: 900 * time.Millisecond},
		{Framework: frameworkPlaywright, Phase: phaseRerun, Duration: 4 * time.Second},
		{Framework: frameworkPlaywright, Phase: phaseRerun, Duration: 5 * time.Second},
	}

	detail := testRunPhaseDetail(t, fixture)

	want := "of which 900ms is 1 measured runner invocation(s)"
	if !strings.Contains(detail, want) {
		t.Errorf("detail = %q, want it to contain %q", detail, want)
	}
}

// TestDiscoveryBoundWithoutDiscoveryRuns checks the honest fallback: a
// run whose only exit records are reruns has nothing measured inside
// the discovery slice, so the slice must claim nothing rather than
// borrow the reruns' duration.
func TestDiscoveryBoundWithoutDiscoveryRuns(t *testing.T) {
	t.Parallel()

	fixture := buildTestFixture()
	fixture.TestRuns = []TestRun{
		{Framework: frameworkPlaywright, Phase: phaseRerun, Duration: 4 * time.Second},
	}

	detail := testRunPhaseDetail(t, fixture)

	if strings.Contains(detail, "of which") {
		t.Errorf("detail = %q, want no measured-time claim", detail)
	}
}

// buildRerunFixture is a `--fix` run: the apply turn lands a fix, the
// engine re-runs the suite to check it, and only then does the next
// validation turn open. The rerun is the shape the timeline used to
// swallow.
func buildRerunFixture() *Fixture {
	fixture := buildTestFixture()
	base := fixture.First

	fixture.Turns = append(fixture.Turns, &Turn{
		Number: 4, Role: rolePrompt, CLI: cliClaude, Model: modelOpus,
		Started: at(base, 240.0), Ended: at(base, 248.0),
		Duration: 8 * time.Second, Status: TurnOK,
	})

	fixture.TestRuns = []TestRun{{
		Framework: frameworkPlaywright,
		Phase:     phaseRerun,
		Duration:  38 * time.Second,
		At:        at(base, 237.0),
		ExitCode:  0,
		HasExit:   true,
	}}

	fixture.Last = at(base, 248.0)
	fixture.Wall = time.Duration(255.62 * float64(time.Second))
	fixture.Judge = &JudgeCall{At: at(base, 252.0)}

	fixture.Phases = BuildPhases(fixture)
	fixture.PhaseTotal = 0

	for _, phase := range fixture.Phases {
		fixture.PhaseTotal += phase.Seconds
	}

	return fixture
}

// findPhase returns the one slice whose label contains want.
func findPhase(t *testing.T, fixture *Fixture, want string) Phase {
	t.Helper()

	for _, phase := range fixture.Phases {
		if strings.Contains(phase.Label, want) {
			return phase
		}
	}

	t.Fatalf("no phase labelled %q in %d slices", want, len(fixture.Phases))

	return Phase{}
}

// TestPostFixRerunIsItsOwnSlice is the point of the split. The engine's
// PostFix hook is what decides whether an applied fix worked, and for a
// webServer-startup subject that is a whole docker-build-and-run suite.
// Reporting it as "Engine between turns" hid the only validating step
// in a `--fix` run behind a label that said prompt-building.
func TestPostFixRerunIsItsOwnSlice(t *testing.T) {
	t.Parallel()

	fixture := buildRerunFixture()

	phase := findPhase(t, fixture, phaseRerun)

	if phase.Owner != OwnerTests {
		t.Errorf("rerun owner = %q, want %q", phase.Owner, OwnerTests)
	}

	if math.Abs(phase.Seconds-38.0) > driftTolerance {
		t.Errorf("rerun slice = %.3fs, want 38.000s", phase.Seconds)
	}

	if !strings.Contains(phase.Label, frameworkPlaywright) {
		t.Errorf("label %q does not name the framework", phase.Label)
	}

	// The verdict is the reason the slice exists — a reader scanning the
	// timeline should see whether the fix held without opening the page.
	if !strings.Contains(phase.Detail, "exit 0") {
		t.Errorf("detail = %q, want the runner's exit status", phase.Detail)
	}
}

// TestRerunTimelineStillSumsAndIsContiguous re-pins the report's central
// invariants against the split gap: carving a slice out of a gap must
// neither lose nor double-count the time either side of it.
func TestRerunTimelineStillSumsAndIsContiguous(t *testing.T) {
	t.Parallel()

	fixture := buildRerunFixture()

	drift := math.Abs(fixture.Wall.Seconds() - fixture.PhaseTotal)
	if drift > driftTolerance {
		t.Errorf("phases sum to %.3fs but wall clock is %.3fs (drift %.3fs)",
			fixture.PhaseTotal, fixture.Wall.Seconds(), drift)
	}

	expected := 0.0

	for _, phase := range fixture.Phases {
		if math.Abs(phase.Offset-expected) > driftTolerance {
			t.Errorf("phase %q starts at %.3fs, expected %.3fs",
				phase.Label, phase.Offset, expected)
		}

		expected += phase.Seconds
	}
}

// TestUnplaceableRerunLeavesGapWhole checks the fallback. A run from a
// log with no exit timestamp cannot be positioned, and inventing a
// position for it would move every later slice on the gantt — so the
// gap stays one undivided engine slice.
func TestUnplaceableRerunLeavesGapWhole(t *testing.T) {
	t.Parallel()

	fixture := buildRerunFixture()
	fixture.TestRuns[0].At = time.Time{}
	fixture.Phases = BuildPhases(fixture)

	for _, phase := range fixture.Phases {
		if strings.Contains(phase.Label, phaseRerun) {
			t.Fatalf("placed an unplaceable run as %q", phase.Label)
		}
	}

	// The post-apply gap is the only large one: 198.137s (apply turn
	// ends) → 240.0s (next turn opens). The others are milliseconds.
	widest := 0.0

	for _, phase := range fixture.Phases {
		if strings.Contains(phase.Label, "Engine between turns") && phase.Seconds > widest {
			widest = phase.Seconds
		}
	}

	if math.Abs(widest-41.863) > driftTolerance {
		t.Errorf("gap = %.3fs, want the whole 41.863s undivided", widest)
	}
}

// TestUnfinishedTurnIsAFloor checks that a turn with no completion
// record is reported as a lower bound rather than as a measurement.
func TestUnfinishedTurnIsAFloor(t *testing.T) {
	base := time.Date(2026, time.August, 9, 20, 0, 0, 0, time.UTC)
	log := &EngineLog{Last: at(base, 30)}
	turns := []*Turn{{Number: 1, Started: base, Status: TurnOpen}}

	log.closeUnfinished(turns)

	if !turns[0].DurationIsFloor {
		t.Error("an unfinished turn should be marked as a lower bound")
	}

	if got := turns[0].Duration.Seconds(); math.Abs(got-30) > driftTolerance {
		t.Errorf("floor duration = %.3fs, want 30s", got)
	}
}
