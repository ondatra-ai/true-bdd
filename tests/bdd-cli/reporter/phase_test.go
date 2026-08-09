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
			Number: 1, Role: rolePrompt, CLI: cliClaude, Model: "opus",
			Started: at(base, 1.046), Ended: at(base, 11.066),
			Duration: 10020 * time.Millisecond, Status: TurnOK,
		},
		{
			Number: 2, Role: roleFix, CLI: cliClaude, Model: "opus",
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
