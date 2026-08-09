package runner

import (
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
// flush closure that finalizes both artifacts and returns their paths.
// Used for prep and teardown, where progress must stay visible during
// long installs but the transcript still has to survive the run.
//
// Output lands on disk as it arrives rather than being held until the
// process exits: a `playwright install` prints tens of megabytes of
// progress, and a prep command killed by the parent test timeout would
// otherwise take its whole transcript with it.
func (s *spawnLog) Tee(name string) (io.Writer, io.Writer, func() (string, string)) {
	stdout := s.open(name, "stdout")
	stderr := s.open(name, "stderr")

	flush := func() (string, string) {
		return stdout.close(), stderr.close()
	}

	return stdout.writer(os.Stdout), stderr.writer(os.Stderr), flush
}

// stream is one live half of a process's output: the artifact file it is
// being written to, or nothing when capture is off or the file could not
// be opened. A stream that cannot be recorded still reaches the console
// — the run's verdict must not depend on the evidence recorder.
type stream struct {
	path string
	file *os.File
}

// open creates the artifact file backing one stream.
func (s *spawnLog) open(name, kind string) stream {
	if s.dir == "" {
		return stream{}
	}

	path := filepath.Join(s.dir, fmt.Sprintf("%s-%s.txt", name, kind))

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, spawnLogFilePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "BDD runner: cannot write %s: %v\n", path, err)

		return stream{}
	}

	return stream{path: path, file: file}
}

// writer fans one stream out to the console and, when an artifact was
// opened, to the file as well.
func (s stream) writer(console io.Writer) io.Writer {
	if s.file == nil {
		return console
	}

	return io.MultiWriter(console, s.file)
}

// close finalizes the artifact and returns where it landed, or "" when
// nothing was recorded.
func (s stream) close() string {
	if s.file == nil {
		return ""
	}

	err := s.file.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "BDD runner: cannot close %s: %v\n", s.path, err)

		return ""
	}

	return s.path
}

// spawnLogName builds a stable, filename-safe basename for one spawn:
// the phase plus its 1-based index within that phase.
func spawnLogName(phase string, idx int) string {
	return fmt.Sprintf("%s-%02d", strings.ReplaceAll(phase, " ", "-"), idx+1)
}
