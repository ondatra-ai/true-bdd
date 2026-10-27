package checklist

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

// ChecklistLoader loads per-command checklist YAMLs from a common directory.
// Each file is single-stage (flat sections at the top).
type ChecklistLoader struct {
	checklistsDir string
}

// NewChecklistLoader creates a loader rooted at `paths.checklists_dir`.
func NewChecklistLoader(cfg *config.ViperConfig) *ChecklistLoader {
	return &ChecklistLoader{
		checklistsDir: cfg.GetString("paths.checklists_dir"),
	}
}

// LoadFull reads the checklist for the named command and returns the full
// Checklist struct, including the optional `config:` block; the runner
// flattens its prompts for the walk.
func (l *ChecklistLoader) LoadFull(commandName string) (*checklist.Checklist, error) {
	path := filepath.Join(l.checklistsDir, commandName+".yaml")
	slog.Debug("Loading full checklist", "command", commandName, "path", path)

	data, err := disk.Read(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read checklist %s: %w", path, pkgerrors.ProjectFile(path, err))
	}

	var parsed checklist.Checklist

	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse checklist %s: %w", path, pkgerrors.ProjectFile(path, err))
	}

	return &parsed, nil
}
