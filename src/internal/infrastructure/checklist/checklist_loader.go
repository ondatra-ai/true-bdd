package checklist

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"bdd-cli/src/internal/domain/models/checklist"
	"bdd-cli/src/internal/infrastructure/config"
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

// Load reads the checklist for the named command (e.g. "us-create") and
// returns its prompts flattened with section context. Skipped prompts are
// filtered out.
func (l *ChecklistLoader) Load(commandName string) ([]checklist.PromptWithContext, error) {
	path := filepath.Join(l.checklistsDir, commandName+".yaml")
	slog.Debug("Loading checklist", "command", commandName, "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read checklist %s: %w", path, err)
	}

	var parsed checklist.Checklist

	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse checklist %s: %w", path, err)
	}

	prompts := make([]checklist.PromptWithContext, 0)

	for _, section := range parsed.Sections {
		for _, prompt := range section.ValidationPrompts {
			if prompt.ShouldSkip() {
				continue
			}

			prompts = append(prompts, checklist.PromptWithContext{
				SectionID:     commandName,
				SectionName:   commandName,
				CriterionID:   section.ID,
				CriterionName: section.Name,
				DefaultDocs:   parsed.DefaultDocs,
				Prompt:        prompt,
			})
		}
	}

	slog.Debug("Checklist loaded",
		"command", commandName,
		"sections", len(parsed.Sections),
		"prompts", len(prompts),
	)

	return prompts, nil
}
