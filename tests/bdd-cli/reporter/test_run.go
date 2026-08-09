package main

import (
	"time"
)

// TestRun is one framework-runner subprocess as the engine recorded it
// after the fact: how it ended, how long it took, and the streams it
// produced.
//
// This is the row's evidence. The engine's parsed view of a run is
// lossy — it becomes a list of failing tests and nothing else — so a
// span that returned zero failures used to be indistinguishable from
// one that never executed. The captured stdout is the framework's own
// report, verbatim, which settles that question without re-running
// anything.
type TestRun struct {
	Framework string
	Phase     string

	ExitCode int
	HasExit  bool
	Duration time.Duration

	// Stdout and Stderr are the persisted streams. A stream that was
	// captured but empty still loads — a zero-byte stderr is the
	// evidence that the framework printed nothing there, which is
	// exactly what a JSON-reporter run looks like.
	Stdout Artifact
	Stderr Artifact

	// Error is set only when the process never started at all.
	Error string
}

// TestRuns lists every framework-runner subprocess of the run, in the
// order they were spawned, with their captured streams loaded.
//
// Driven by the exit record rather than the spawn record: a spawn is
// written before the fork and so proves only intent, while the exit
// record is written after the process is gone and carries what it did.
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

	return "exited " + itoa(t.ExitCode) +
		" — routine for a framework reporting test failures"
}
