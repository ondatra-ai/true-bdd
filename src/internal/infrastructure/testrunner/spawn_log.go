package testrunner

import "log/slog"

// logSpawn records the exact command a framework runner is about to
// execute.
//
// Discovery is otherwise invisible in the log: the engine writes nothing
// between loading the architecture and loading the checklist, so a run
// report can bound the test-run span but cannot say what ran inside it.
// The record uses the same shape the AI providers use for their own
// subprocesses ("Spawning agent CLI"), so one parser covers both.
func logSpawn(binary string, args []string, dir string) {
	slog.Debug("Spawning test runner",
		"binary", binary,
		"args", args,
		"dir", dir,
	)
}
