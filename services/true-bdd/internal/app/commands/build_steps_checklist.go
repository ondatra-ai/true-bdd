package commands

import (
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

const buildStepsChecklistName = "build-steps"

func buildStepsPrompts(
	loader *checklist.ChecklistLoader, resolver *docs.Resolver,
) ([]checklistmodels.PromptWithContext, error) {
	doc, err := loader.LoadFull(buildStepsChecklistName)
	if err != nil {
		return nil, err
	}

	prompts := make([]checklistmodels.PromptWithContext, 0)

	for _, section := range doc.Sections {
		for _, prompt := range section.ValidationPrompts {
			err = prompt.ShapeError()
			if err != nil {
				return nil, err
			}

			if skipBuildStepPrompt(prompt) {
				continue
			}

			if prompt.Check != "step-coverage" || len(prompt.With) > 0 {
				return nil, ErrBuildStepsChecklist
			}

			prompts = append(prompts, checklistmodels.PromptWithContext{
				SectionID: buildStepsChecklistName, CriterionID: section.ID, Index: len(prompts) + 1,
				Prompt: prompt, DefaultDocs: doc.DefaultDocs, DefaultModels: doc.Engine,
			})
		}
	}

	if len(prompts) == 0 {
		return nil, ErrBuildStepsChecklist
	}

	err = resolveStepCoverageDocs(prompts, resolver)
	if err != nil {
		return nil, err
	}

	return prompts, nil
}

func recordStepCoverage(
	tmpDir string, prompts []checklistmodels.PromptWithContext, findings []string, missing int, logPrompts bool,
) error {
	if logPrompts {
		slog.Info("Loaded prompts", "command", "build steps", "items", 1, "prompts", len(prompts))
	}

	context := append([]string{fmt.Sprintf("%d missing phrase definition(s)", missing)}, findings...)

	verdict := checks.Verdict{Pass: missing == 0 && len(findings) == 0, Context: context}
	for _, prompt := range prompts {
		result := validate.CheckResult(prompt, verdict)
		slog.Info("Check answered", "check", prompt.Prompt.Check, "subjectID", "registry",
			"promptIndex", prompt.Index, "answer", result.ActualAnswer, "context", result.Context)

		err := validate.SaveCheckVerdict(tmpDir, "registry", result)
		if err != nil {
			return fmt.Errorf("save Step coverage: %w", err)
		}
	}

	return nil
}

func resolveStepCoverageDocs(prompts []checklistmodels.PromptWithContext, resolver *docs.Resolver) error {
	for _, prompt := range prompts {
		_, err := resolver.ResolveAll(prompt.GetEffectiveDocs())
		if err != nil {
			return fmt.Errorf("build steps checklist documents: %w", err)
		}
	}

	return nil
}

func skipBuildStepPrompt(prompt checklistmodels.Prompt) bool {
	return prompt.ShouldSkip() || prompt.With["concepts"] == "group" || prompt.With["concepts"] == "propose"
}
