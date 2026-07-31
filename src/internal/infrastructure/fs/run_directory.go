package fs

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/src/internal/pkg/errors"
)

const fileModeDirectory = 0755 // Standard directory permission

// RunDirectory manages timestamped run directories for organizing tmp files.
type RunDirectory struct {
	runPath string
}

// NewRunDirectory creates a new timestamped run directory.
// Format: basePath/YYYY-MM-DD-HH-MM-SS-<pid>. Seconds + the process id
// keep sequential runs (and concurrent harness-dispatched children in the
// same folder) from colliding on the minute-only name the engine used
// before (plan §3.2).
func NewRunDirectory(basePath string) (*RunDirectory, error) {
	dirName := runDirName(time.Now(), os.Getpid())
	runPath := filepath.Join(basePath, dirName)

	err := os.MkdirAll(runPath, fileModeDirectory)
	if err != nil {
		slog.Error("Failed to create run directory", "path", runPath, "error", err)

		return nil, errors.ErrCreateRunDirectoryFailed(err)
	}

	return &RunDirectory{
		runPath: runPath,
	}, nil
}

// GetTmpOutPath returns the full path to the run directory.
func (rd *RunDirectory) GetTmpOutPath() string {
	return rd.runPath
}

// runDirName builds the run-directory basename from a wall-clock time and
// a process id. Distinct (time-to-the-second, pid) pairs yield distinct
// names, so two harness-dispatched children in the same folder — separate
// processes, hence distinct pids — never share a run directory even within
// the same second (plan §3.2).
func runDirName(now time.Time, pid int) string {
	return fmt.Sprintf("%s-%d", now.Format("2006-01-02-15-04-05"), pid)
}
