package commands

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/verification"
)

func (e *buildStepsExecution) critique(
	ctx context.Context, item *BuildStepsItem, prompt *checklistmodels.PromptWithContext,
) ([]validate.CritiqueFinding, error) {
	sources, err := verification.Sources(item.Suite, item.functions())
	if err != nil {
		return nil, err
	}

	evidence, err := critiqueContext(item, sources)
	if err != nil {
		return nil, err
	}

	arguments, err := architecture.LoadStepArguments(e.architectureFile)
	if err != nil {
		return nil, err
	}

	artifact := validate.CellArtifactStem(e.deps.TmpDir, item.subjectID, *prompt)

	findings, err := e.deps.Critic.Critique(ctx, validate.StepCritiqueData{Question: prompt.Prompt.Question,
		StepArguments: arguments, Definition: definitionLabels(item), Context: evidence, Sources: sources,
	}, prompt.EffectiveModelTier(), artifact)
	if err != nil {
		return nil, err
	}

	slog.Info("Step definition verified", "definition", strings.Join(item.functions(), ","), "findings", findings)

	return findings, nil
}

func (e *buildStepsExecution) prepareCritique() error {
	checklist, err := e.deps.ChecklistLoader.LoadFull(buildStepsChecklistName)
	if err != nil {
		return err
	}

	for _, section := range checklist.Sections {
		if section.ID != stepVerificationID {
			continue
		}

		for _, prompt := range section.ValidationPrompts {
			if prompt.ShouldSkip() {
				continue
			}

			tier := (&checklistmodels.PromptWithContext{Prompt: prompt, DefaultModels: checklist.Engine}).EffectiveModelTier()

			return e.deps.Critic.Validate(tier)
		}
	}

	return nil
}

func (e *buildStepsExecution) answerVerification(
	ctx context.Context, item *BuildStepsItem, prompt *checklistmodels.PromptWithContext,
) (checklistmodels.ValidationResult, error) {
	if prompt.CriterionID != stepVerificationID {
		return checklistmodels.ValidationResult{}, ErrBuildStepsChecklist
	}

	findings, err := e.critique(ctx, item, prompt)
	if err != nil {
		return checklistmodels.ValidationResult{}, err
	}

	if e.verification == nil {
		e.verification = make(map[*BuildStepsItem][]validate.CritiqueFinding)
	}

	e.verification[item] = findings
	delete(e.fixItems, item)

	lines := make([]string, 0, len(findings))
	for _, finding := range findings {
		lines = append(lines, finding.String())
	}

	result := validate.CheckResult(*prompt, checks.Verdict{Pass: len(findings) == 0, Context: lines})
	result.Question = prompt.Prompt.Question

	return result, validate.SaveStepVerdict(e.deps.TmpDir, item.subjectID, *prompt, findings, result.ActualAnswer)
}
