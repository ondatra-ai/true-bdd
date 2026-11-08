package commands

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

const scenarioRunID = "scenario-run"

func asksBuildSteps(item *BuildStepsItem, prompt *checklist.PromptWithContext) bool {
	return (item.Concept == nil) == (prompt.Prompt.Check == scenarioRunID)
}

func scenarioRunItems(scenarios []*registry.RegistryScenario, arch *architecture.Architecture) []*BuildStepsItem {
	if arch.Testing.Testkit == nil {
		return nil
	}

	items := make([]*BuildStepsItem, 0)
	seen := make(map[string]bool)

	for _, scenario := range scenarios {
		dir := filepath.Join(filepath.Dir(scenario.Path), "steps")
		if seen[dir] {
			continue
		}

		seen[dir] = true
		items = append(items, &BuildStepsItem{Suite: dir, Service: scenario.Service, subjectID: dir + "-scenario-run"})
	}

	return items
}

func scenarioRunVerdict(item *BuildStepsItem, _ checks.Input) (checks.Verdict, error) {
	file, present, err := stepskeleton.InspectScenarioRun(item.Suite)
	if err != nil {
		return checks.Verdict{}, err
	}

	answer := "State embeds *testkit.ScenarioRun"
	if !present {
		answer = "State must embed *testkit.ScenarioRun"
	}

	return checks.Verdict{Pass: present, Context: []string{fmt.Sprintf("%s: %s: %s", item.Label(), file, answer)}}, nil
}

func (e *buildStepsExecution) generateStepFix(ctx context.Context, item *BuildStepsItem,
	failed checklist.ValidationResult, answers map[string]string, iteration int,
) (engine.FixResult, bool, error) {
	if failed.Check != scenarioRunID {
		return e.generatePatternFix(ctx, item, failed, answers, iteration)
	}

	file, _, err := stepskeleton.InspectScenarioRun(item.Suite)
	if err != nil {
		return engine.FixResult{}, true, err
	}

	prompt := fmt.Sprintf("%s\nSuite: %s\nFile: %s", failed.FixPrompt, item.Suite, file)

	err = validate.SaveCodeFix(e.deps.TmpDir, item.subjectID, failed, prompt, iteration)

	return engine.FixResult{FixPrompt: prompt}, true, err
}

func (e *buildStepsExecution) applyStepFix(ctx context.Context, item *BuildStepsItem,
	failed checklist.ValidationResult, decision engine.FixDecision,
) (bool, error) {
	if failed.Check != scenarioRunID {
		return e.applyPatternFix(ctx, item, failed, decision)
	}

	err := stepskeleton.EnsureScenarioRun(item.Suite)
	if err != nil {
		return true, err
	}

	e.scenarioRunApplies++

	return true, validate.SaveCodeApply(e.deps.TmpDir, item.subjectID, e.scenarioRunApplies)
}

func recordScenarioRuns(deps BuildStepsDeps, items []*BuildStepsItem,
	prompt checklist.PromptWithContext,
) ([]string, error) {
	findings := make([]string, 0)

	for _, item := range items {
		if !asksBuildSteps(item, &prompt) {
			continue
		}

		verdict, err := scenarioRunVerdict(item, checks.Input{})
		if err != nil {
			return nil, err
		}

		result := validate.CheckResult(prompt, verdict)
		slog.Info("Check answered", "check", scenarioRunID, "subjectID", item.subjectID,
			"promptIndex", prompt.Index, "answer", result.ActualAnswer, "context", result.Context)

		err = validate.SaveCheckVerdict(deps.TmpDir, item.subjectID, result)
		if err != nil {
			return nil, err
		}

		if !verdict.Pass {
			findings = append(findings, verdict.Context...)
		}
	}

	return findings, nil
}
