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

// Artifacts persists the raw streams of every test-runner subprocess into
// the engine's run directory. The parsed report is lossy by design (it
// collapses to a list of FailingTest values), so these files are what lets a finished run be audited instead of re-run.
type Artifacts struct {
	dir string
	seq atomic.Uint64
}

// NewArtifacts builds a writer rooted at dir, the engine's run directory.
// One instance is shared by every framework runner, so sequence numbers
// order all of a run's test-runner spawns together, not restarting per framework.
func NewArtifacts(dir string) *Artifacts {
	return &Artifacts{dir: dir}
}

// Capture writes one subprocess's stdout/stderr and returns their paths.
// A nil receiver (a runner built without a run directory) captures nothing.
// Empty streams are still written — that's what made a Playwright startup failure look like missing output.
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

// write persists one stream, returning its path or "" if it could not be
// written. Failures are logged and swallowed: capturing evidence is worth
// a warning, never worth failing a test run that otherwise completed.
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
