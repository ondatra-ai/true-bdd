package reporter

import (
	"strconv"
	"time"
)

const (
	// phaseDiscover is the engine's label for the opening whole-suite run
	// (testrunner.PhaseDiscover). Everything else is a fix-loop rerun of
	// a single test, which happens long after the discovery slice closed.
	phaseDiscover = "discover"
	// phaseRerun is the engine's label for one test re-executed after a
	// fix was applied (testrunner.PhaseRerun).
	phaseRerun = "rerun"
)

// TestRun is one framework-runner subprocess as the engine recorded it: how
// it ended, how long it took, and its streams. The engine's own parsed view
// is lossy (just a list of failures), so stdout settles zero-failures-vs-never-executed.
type TestRun struct {
	Framework string
	Phase     string

	ExitCode int
	HasExit  bool
	Duration time.Duration

	// At is when the exit record was written; with Duration it places the run
	// on the wall clock, letting a rerun show as its own timeline slice.
	At time.Time

	// Stdout and Stderr are the persisted streams. A captured-but-empty
	// stream still loads — a zero-byte stderr is evidence the framework
	// printed nothing, not that nothing was captured.
	Stdout Artifact
	Stderr Artifact

	// Error is set only when the process never started at all.
	Error string
}

// TestRuns lists every framework-runner subprocess of the run, in the order
// they were spawned, with their captured streams loaded. Driven by the exit
// record, not the spawn record: a spawn only proves intent.
func (l *EngineLog) TestRuns(fixtureDir string) []TestRun {
	var runs []TestRun

	for index := range l.Records {
		record := &l.Records[index]
		if record.Msg != msgRunnerReturned {
			continue
		}

		runs = append(runs, newTestRun(record, fixtureDir))
	}

	return runs
}

// newTestRun folds one exit record into a TestRun, loading whichever
// streams it named.
func newTestRun(record *LogRecord, fixtureDir string) TestRun {
	run := TestRun{
		Framework: record.Framework,
		Phase:     record.Phase,
		Error:     record.Error,
		At:        record.At,
	}

	if record.ExitCode != nil {
		run.ExitCode = *record.ExitCode
		run.HasExit = true
	}

	if record.DurationMs != nil {
		run.Duration = time.Duration(*record.DurationMs) * time.Millisecond
	}

	if record.StdoutFile != "" {
		run.Stdout = loadArtifact(fixtureDir, record.StdoutFile)
	}

	if record.StderrFile != "" {
		run.Stderr = loadArtifact(fixtureDir, record.StderrFile)
	}

	return run
}

// IsDiscovery reports whether the run is the opening whole-suite invocation
// — the one the timeline's pre-dispatch slice is bounded by. Every other
// phase is a fix-loop rerun, with its own slice (see appendGap).
func (t TestRun) IsDiscovery() bool {
	return t.Phase == phaseDiscover
}

// Started is when the subprocess began, derived from the exit record
// and the wall clock it reported. Zero when either is missing, which is
// the signal that the run cannot be placed on the timeline.
func (t TestRun) Started() time.Time {
	if t.At.IsZero() {
		return time.Time{}
	}

	return t.At.Add(-t.Duration)
}

// Label names the run for a section heading: the framework plus which
// walk phase it belongs to, so a fix loop's re-runs are told apart from
// the opening discovery.
func (t TestRun) Label() string {
	switch {
	case t.Framework == "" && t.Phase == "":
		return "test runner"
	case t.Phase == "":
		return t.Framework
	case t.Framework == "":
		return t.Phase
	default:
		return t.Framework + " · " + t.Phase
	}
}

// Outcome states how the process ended, in the terms that distinguish
// the cases a bare duration cannot: a clean pass, a framework reporting
// failures through its exit code, or a process that never ran.
func (t TestRun) Outcome() string {
	if t.Error != "" {
		return "never started — " + t.Error
	}

	if !t.HasExit {
		return ""
	}

	if t.ExitCode == 0 {
		return "exited 0"
	}

	return "exited " + strconv.Itoa(t.ExitCode) +
		" — routine for a framework reporting test failures"
}
