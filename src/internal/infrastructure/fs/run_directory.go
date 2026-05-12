package fs

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"bdd-cli/src/internal/pkg/errors"
)

const fileModeDirectory = 0755 // Standard directory permission

// RunDirectory manages timestamped run directories for organizing tmp files.
type RunDirectory struct {
	runPath string
}

// NewRunDirectory creates a new timestamped run directory
// Format: basePath/YYYY-MM-DD-HH-MM where HH-MM is hours and minutes.
func NewRunDirectory(basePath string) (*RunDirectory, error) {
	// Format: YYYY-MM-DD-HH-MM
	timestamp := time.Now().Format("2006-01-02-15-04")
	dirName := timestamp
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
