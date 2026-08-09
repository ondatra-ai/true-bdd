package runner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SpawnLogDir is the tmpdir subdirectory holding the harness's own
// spawn evidence: the streams of every process the runner starts.
//
// It lives inside the tmpdir so evidence sits next to the run it
// explains, but every write is timed to fall outside the before/after
// snapshot window — prep before the "before" snapshot, the CLI and
// teardown after the "after" — so nothing here reaches the diff the
// judge grades.
const SpawnLogDir = "bdd-cli-logs"

const (
	spawnLogDirPerm  = 0755
	spawnLogFilePerm = 0644
)

// spawnLog captures one harness-spawned process's streams under the
// tmpdir. The engine writes its own subprocess evidence into its run
// directory; this is the same idea one level up, for the processes the
// engine never sees — the CLI under test and the fixture's prep and
// teardown shells.
type spawnLog struct {
	dir string
}

// newSpawnLog roots a capture directory inside tmpDir. A directory that
// cannot be created disables capture rather than failing the fixture:
// the run's verdict must not depend on the evidence recorder.
func newSpawnLog(tmpDir string) *spawnLog {
	dir := filepath.Join(tmpDir, SpawnLogDir)

	err := os.MkdirAll(dir, spawnLogDirPerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "BDD runner: cannot create %s: %v\n", dir, err)

		return &spawnLog{}
	}

	return &spawnLog{dir: dir}
}

// Write persists one stream under `<name>-<stream>.txt` and returns the
// path it landed at, or "" when capture is disabled or the write failed.
func (s *spawnLog) Write(name, stream string, payload []byte) string {
	if s.dir == "" {
		return ""
	}

	path := filepath.Join(s.dir, fmt.Sprintf("%s-%s.txt", name, stream))

	err := os.WriteFile(path, payload, spawnLogFilePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "BDD runner: cannot write %s: %v\n", path, err)

		return ""
	}

	return path
}

// Tee returns writers that copy a live process's streams to disk while
// still passing them through to the harness's own stdout/stderr, plus a
// flush closure that persists what was captured and returns the two
// paths. Used for prep and teardown, where progress must stay visible
// during long installs but the transcript still has to survive the run.
func (s *spawnLog) Tee(name string) (io.Writer, io.Writer, func() (string, string)) {
	var stdout, stderr bytes.Buffer

	flush := func() (string, string) {
		return s.Write(name, "stdout", stdout.Bytes()),
			s.Write(name, "stderr", stderr.Bytes())
	}

	return io.MultiWriter(os.Stdout, &stdout),
		io.MultiWriter(os.Stderr, &stderr),
		flush
}

// spawnLogName builds a stable, filename-safe basename for one spawn:
// the phase plus its 1-based index within that phase.
func spawnLogName(phase string, idx int) string {
	return fmt.Sprintf("%s-%02d", strings.ReplaceAll(phase, " ", "-"), idx+1)
}
