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
const stepVerificationID = "step-verification"
const stepCoverageID = "step-coverage"

// scenCheckChecklistName is the checklist whose bindings Prompt build steps judges bindings with.
const scenCheckChecklistName = "scen-check"

func buildStepsPrompts(
	loader *checklist.ChecklistLoader, resolver *docs.Resolver,
) ([]checklistmodels.PromptWithContext, error) {
	doc, err := loader.LoadFull(buildStepsChecklistName)
	if err != nil {
		return nil, err
	}

	prompts := make([]checklistmodels.PromptWithContext, 0)

	index := 0

	for _, section := range doc.Sections {
		for _, prompt := range section.ValidationPrompts {
			index++

			err = prompt.ShapeError()
			if err != nil {
				return nil, err
			}

			if skipBuildStepPrompt(prompt) {
				continue
			}

			if !walkableStepPrompt(section.ID, prompt) {
				return nil, ErrBuildStepsChecklist
			}

			prompts = append(prompts, checklistmodels.PromptWithContext{
				SectionID: buildStepsChecklistName, CriterionID: section.ID, Index: index,
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
	tmpDir string, prompt checklistmodels.PromptWithContext, findings []string, missing int,
) error {
	context := append([]string{fmt.Sprintf("%d concept Item(s) without step definitions", missing)}, findings...)

	verdict := checks.Verdict{Pass: missing == 0 && len(findings) == 0, Context: context}

	result := validate.CheckResult(prompt, verdict)
	slog.Info("Check answered", "check", prompt.Prompt.Check, "subjectID", "registry",
		"promptIndex", prompt.Index, "answer", result.ActualAnswer, "context", result.Context)

	err := validate.SaveCheckVerdict(tmpDir, "registry", result)
	if err != nil {
		return fmt.Errorf("save Step coverage: %w", err)
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
	return prompt.ShouldSkip() || prompt.Judgement() == checklistmodels.JudgementGroup ||
		prompt.Judgement() == checklistmodels.JudgementPropose
}

func walkableStepPrompt(section string, prompt checklistmodels.Prompt) bool {
	if len(prompt.With) > 0 {
		return false
	}

	return prompt.Check == scenarioRunID || prompt.Check == stepCoverageID || prompt.Check == patternBindingsID ||
		(!prompt.IsCheck() && section == stepVerificationID)
}
