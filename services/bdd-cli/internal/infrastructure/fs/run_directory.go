package fs

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

// RunDirectory manages timestamped run directories for organizing tmp files.
type RunDirectory struct {
	runPath string
}

// NewRunDirectory creates a new timestamped run directory, formatted
// basePath/YYYY-MM-DD-HH-MM-SS-<pid>; see runDirName for why seconds+pid
// avoid collisions (plan §3.2).
func NewRunDirectory(basePath string) (*RunDirectory, error) {
	dirName := runDirName(time.Now(), os.Getpid())
	runPath := filepath.Join(basePath, dirName)

	err := disk.Dir(runPath, disk.Shared)
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

// runDirName builds the run-directory basename from a wall-clock time and a
// pid: distinct (second, pid) pairs give distinct names, so two children in
// the same folder never collide even within the same second (plan §3.2).
func runDirName(now time.Time, pid int) string {
	return fmt.Sprintf("%s-%d", now.Format("2006-01-02-15-04-05"), pid)
}
