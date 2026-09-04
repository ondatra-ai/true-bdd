package reporter

import (
	"math"
	"strings"
	"testing"
	"time"
)

// driftTolerance is how far the accounted timeline may land from the wall
// clock before the phase model is considered to have a hole; slices are
// millisecond-resolution, so small rounding is expected.
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

	fixture.Phases = buildPhases(fixture)
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

// TestPhaseOwnership checks the test subprocess is not counted as engine
// time — conflating them would report a slow project test suite as slow
// engine code, the reason the Go-side breakdown exists.
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

	for _, phase := range buildPhases(fixture) {
		if phase.Owner == OwnerTests {
			return phase.Detail
		}
	}

	t.Fatal("no test-run phase in the timeline")

	return ""
}

// TestDiscoveryBoundCountsOnlyDiscoveryRuns pins discoveryBound's filter:
// only discovery invocations count, not the `--fix` loop's reruns that
// share the same list.
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

// TestDiscoveryBoundWithoutDiscoveryRuns checks the honest fallback: with
// no discovery-phase runs, the slice claims nothing measured rather than
// borrowing a rerun's duration.
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
// validation turn open.
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

	fixture.Phases = buildPhases(fixture)
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

// TestPostFixRerunIsItsOwnSlice is the point of the split documented on
// appendGap: the PostFix rerun gets its own slice instead of hiding inside
// "Engine between turns".
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

// TestTestRunSlicesNameTheirOwnRuns pins Phase.TestRuns: two reruns of the
// same test produce byte-identical labels, so only the index tells the
// slices apart.
func TestTestRunSlicesNameTheirOwnRuns(t *testing.T) {
	t.Parallel()

	fixture := buildTestFixture()
	base := fixture.First

	// Two more validation turns, so there are two wide gaps for a rerun
	// to land in rather than the 1ms boundaries the base fixture has.
	fixture.Turns = append(fixture.Turns,
		&Turn{
			Number: 4, Role: rolePrompt, CLI: cliClaude, Model: modelOpus,
			Started: at(base, 240.0), Ended: at(base, 248.0),
			Duration: 8 * time.Second, Status: TurnOK,
		},
		&Turn{
			Number: 5, Role: rolePrompt, CLI: cliClaude, Model: modelOpus,
			Started: at(base, 300.0), Ended: at(base, 308.0),
			Duration: 8 * time.Second, Status: TurnOK,
		})

	// Two reruns, identical in every field a label is built from, landing
	// in the two different gaps.
	fixture.TestRuns = []TestRun{
		{
			Framework: frameworkPlaywright, Phase: phaseRerun,
			Duration: 2 * time.Second, At: at(base, 210.0), HasExit: true,
		},
		{
			Framework: frameworkPlaywright, Phase: phaseRerun,
			Duration: 2 * time.Second, At: at(base, 260.0), HasExit: true,
		},
	}

	fixture.Last = at(base, 308.0)
	fixture.Phases = buildPhases(fixture)

	// Only the rerun slices — the base fixture also has a discovery slice,
	// which claims no runs here because none are discovery-phase.
	var got [][]int

	for _, phase := range fixture.Phases {
		if strings.Contains(phase.Label, phaseRerun) {
			got = append(got, phase.TestRuns)
		}
	}

	if len(got) != 2 {
		t.Fatalf("got %d rerun slices, want 2", len(got))
	}

	// Each slice claims exactly one run, and they claim different ones.
	for index, runs := range got {
		if len(runs) != 1 {
			t.Fatalf("slice %d claims %v, want exactly one run", index, runs)
		}

		if runs[0] != index {
			t.Errorf("slice %d claims run %d, want run %d", index, runs[0], index)
		}
	}
}

// TestDiscoverySliceClaimsEveryDiscoveryRun pins the other half: the
// opening slice is bounded by log records rather than by one
// subprocess, so it legitimately contains all of them and must say so.
func TestDiscoverySliceClaimsEveryDiscoveryRun(t *testing.T) {
	t.Parallel()

	fixture := buildTestFixture()
	fixture.TestRuns = []TestRun{
		{Framework: frameworkPlaywright, Phase: phaseDiscover, Duration: time.Second},
		{Framework: frameworkPlaywright, Phase: phaseDiscover, Duration: time.Second},
		{Framework: frameworkPlaywright, Phase: phaseRerun, Duration: time.Second},
	}

	fixture.Phases = buildPhases(fixture)

	phase := findPhase(t, fixture, "Test run")

	if len(phase.TestRuns) != 2 {
		t.Fatalf("discovery slice claims %v, want the two discovery runs", phase.TestRuns)
	}

	// The rerun is not a discovery run and must not be swept in — it gets
	// its own slice in the gap it actually ran in.
	for _, index := range phase.TestRuns {
		if !fixture.TestRuns[index].IsDiscovery() {
			t.Errorf("discovery slice claims run %d, which is phase %q",
				index, fixture.TestRuns[index].Phase)
		}
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

// TestUnplaceableRerunLeavesGapWhole checks the fallback: a run with no
// exit timestamp can't be positioned, so the gap stays one undivided
// engine slice rather than moving every later slice on the gantt.
func TestUnplaceableRerunLeavesGapWhole(t *testing.T) {
	t.Parallel()

	fixture := buildRerunFixture()
	fixture.TestRuns[0].At = time.Time{}
	fixture.Phases = buildPhases(fixture)

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
