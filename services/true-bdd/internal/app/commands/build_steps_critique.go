package commands

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func (e *buildStepsExecution) critique(ctx context.Context, item *BuildStepsItem) ([]validate.CritiqueFinding, error) {
	paths, err := filepath.Glob(filepath.Join(item.Suite, "*.go"))
	if err != nil {
		return nil, fmt.Errorf("list definition sources: %w", err)
	}

	var sources strings.Builder

	for _, path := range paths {
		content, err := disk.Read(path)
		if err != nil {
			return nil, fmt.Errorf("read definition source: %w", err)
		}

		fmt.Fprintf(&sources, "\n--- %s ---\n%s\n", path, content)
	}

	artifact := filepath.Join(e.deps.TmpDir, fmt.Sprintf("critique-%s-%d", item.subjectID, e.attempts[item]))

	findings, err := e.deps.Critic.Critique(ctx, validate.StepCritiqueData{
		Definition: definitionLabels(item), Context: item.Context, Sources: sources.String(),
		Concept: item.Concept != nil,
	}, e.critiqueTier, artifact)
	if err != nil {
		return nil, err
	}

	slog.Info("Step definition critiqued", "definition", strings.Join(item.functions(), ","), "findings", findings)

	return findings, nil
}

func (e *buildStepsExecution) prepareCritique() error {
	checklist, err := e.deps.ChecklistLoader.LoadFull(buildStepsChecklistName)
	if err != nil {
		return err
	}

	if checklist.Engine != nil {
		e.critiqueTier = checklist.Engine.CritiqueModel
	}

	return e.deps.Critic.Validate(e.critiqueTier)
}
