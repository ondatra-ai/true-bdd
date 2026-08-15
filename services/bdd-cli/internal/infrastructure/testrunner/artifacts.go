package testrunner

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
)

// fileModeArtifact is the permission for a persisted runner stream.
// Matches the mode the engine uses for its prompt/response artifacts.
const fileModeArtifact = 0644

// Artifacts persists the raw streams of every test-runner subprocess
// into the engine's run directory, alongside the prompts the engine
// built from them.
//
// The parsed report is lossy by design: it collapses into a list of
// FailingTest values, and everything else — the run-level errors, the
// per-test timings, whatever the framework printed on the way — is
// dropped as soon as the walk begins. Keeping both streams verbatim is
// what lets a finished run be audited instead of re-run: the exit
// record says a suite failed, these files say what it said.
type Artifacts struct {
	dir string
	seq atomic.Uint64
}

// NewArtifacts builds a writer rooted at dir, the engine's run
// directory. One instance is shared by every framework runner, so the
// sequence numbers order all of a run's test-runner spawns against each
// other rather than restarting per framework.
func NewArtifacts(dir string) *Artifacts {
	return &Artifacts{dir: dir}
}

// Capture writes one subprocess's stdout and stderr and returns their
// paths for the exit log record. A nil receiver — a runner built
// without a run directory, as in unit tests — captures nothing and
// returns empty paths.
//
// Empty streams are still written: a zero-byte stderr file is the
// evidence that the framework really did print nothing there, which is
// exactly the fact that made Playwright's startup failures look like
// missing output rather than output in the other stream.
func (a *Artifacts) Capture(
	framework, phase string,
	stdout, stderr *bytes.Buffer,
) (string, string) {
	if a == nil || a.dir == "" {
		return "", ""
	}

	base := fmt.Sprintf("testrun-%03d-%s-%s", a.seq.Add(1), framework, phase)

	stdoutPath := a.write(base+"-stdout."+stdoutExtension(framework), stdout.Bytes())
	stderrPath := a.write(base+"-stderr.txt", stderr.Bytes())

	return stdoutPath, stderrPath
}

// write persists one stream, returning the path it landed at or "" if
// it could not be written. Failures are logged and swallowed: capturing
// evidence is worth a warning, never worth failing a test run that
// otherwise completed.
func (a *Artifacts) write(name string, payload []byte) string {
	path := filepath.Join(a.dir, name)

	err := os.WriteFile(path, payload, fileModeArtifact)
	if err != nil {
		slog.Warn("Failed to persist test runner output",
			"file", path,
			"error", err,
		)

		return ""
	}

	return path
}

// stdoutExtension names the captured stdout file after what the
// framework actually writes there: one JSON document for playwright and
// jest, a JSON-per-line event stream for `go test -json`.
func stdoutExtension(framework string) string {
	if framework == FrameworkGoTest {
		return "jsonl"
	}

	return jsonToken
}
