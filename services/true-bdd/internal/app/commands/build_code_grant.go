package commands

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

func configureReplayGrant(applier *validate.FixApplier, testing architecture.Testing) {
	applier.UseTestCommand(nil)

	command, selector, err := testrunner.CanonicalReplayCommand(testing.Framework, testing.Commands.Replay)
	if err != nil {
		slog.Warn("Replay command grant unavailable", "reason", err)

		return
	}

	root, err := os.Getwd()
	if err != nil {
		slog.Warn("Replay command grant unavailable", "reason", err)

		return
	}

	dir := testrunner.CommandDir(testrunner.Config{ConfigFile: testing.ConfigFile})
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}

	applier.UseTestCommand(&provider.TestCommand{Command: command, Selector: selector, Dir: dir})
}
