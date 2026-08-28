package bootstrap

import (
	"log/slog"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
)

// engineLogFile is the JSON log tests/libraries/reporter reads back.
const engineLogFile = "true-bdd.log.json"

// configureLogging binds the engine's stream. Stdout, not stderr: 22 steps in
// docs/scenarios.yaml assert `stdout matches level=ERROR msg="Refusing to
// start"`, and the registry has no stderr step at all.
func configureLogging(tmpDir string) {
	err := disk.Dir(tmpDir, disk.Shared)
	if err != nil {
		slog.Warn("Failed to create tmp directory", "error", err)
	}

	logging.Install(logging.Stdout, filepath.Join(tmpDir, engineLogFile), "")
}
