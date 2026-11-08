package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
)

const patternBindingsID = "pattern-bindings"

func (e *buildStepsExecution) patternBindings(item *BuildStepsItem, _ checks.Input) (checks.Verdict, error) {
	delete(e.fixItems, item)
	subject := e.concepts.repairSubject(item, nil)
	findings := catalogue.CheckPatterns(subject.ID, item.Concept.registeredPatterns(), subject.Suites)

	return checks.Verdict{Pass: len(findings) == 0, Context: findings}, nil
}

func (e *buildStepsExecution) generatePatternFix(ctx context.Context, item *BuildStepsItem,
	failed checklist.ValidationResult, answers map[string]string, iteration int,
) (engine.FixResult, bool, error) {
	if failed.Check != patternBindingsID {
		return engine.FixResult{}, false, nil
	}

	subject := e.concepts.repairSubject(item, nil)

	subject.Findings = append([]string{}, failed.Context...)
	if candidate := e.fixItems[item]; candidate != nil {
		subject.Previous = candidate.Concept.Patterns
	}

	for _, key := range slices.Sorted(maps.Keys(answers)) {
		subject.Findings = append(subject.Findings, answers[key])
	}

	candidate, concept := *item, *item.Concept
	candidate.Concept = &concept

	err := e.proposeConcept(ctx, e.concepts.proposal, &candidate, subject, fmt.Sprintf("pattern fix %d", iteration))
	if err != nil {
		return engine.FixResult{}, true, err
	}

	if e.fixItems == nil {
		e.fixItems = make(map[*BuildStepsItem]*BuildStepsItem)
	}

	e.fixItems[item] = &candidate

	encoded, err := json.MarshalIndent(candidate.Concept.Patterns, "", "  ")
	if err != nil {
		return engine.FixResult{}, true, fmt.Errorf("render proposed patterns: %w", err)
	}

	return engine.FixResult{FixPrompt: "Rewrite this concept's skeletons with the proposed patterns:\n" +
		string(encoded)}, true, nil
}

func (e *buildStepsExecution) applyPatternFix(_ context.Context, item *BuildStepsItem,
	failed checklist.ValidationResult, _ engine.FixDecision,
) (bool, error) {
	if failed.Check != patternBindingsID {
		return false, nil
	}

	candidate := e.fixItems[item]
	if candidate == nil {
		return true, ErrBuildStepsChecklist
	}

	item.Concept.Patterns = candidate.Concept.Patterns
	item.Findings = nil
	delete(e.fixItems, item)

	return true, e.rewriteSkeletons(item)
}

// registeredPatterns supplies only resumed registrations' capture cardinality; proposals keep their declared names.
func (concept *BuildStepsConcept) registeredPatterns() []catalogue.ProposedPattern {
	patterns := concept.proposed()
	for index := range patterns {
		if slices.ContainsFunc(concept.Patterns, func(proposed catalogue.ProposedPattern) bool {
			return proposed.Pattern == patterns[index].Pattern
		}) {
			continue
		}

		compiled, err := regexp.Compile(patterns[index].Pattern)
		if err == nil {
			patterns[index].Captures = make([]catalogue.PatternCapture, compiled.NumSubexp())
		}
	}

	return patterns
}

func recordBarePatternBindings(deps BuildStepsDeps, discovery *conceptDiscovery,
	prompt checklist.PromptWithContext,
) error {
	execution := &buildStepsExecution{concepts: &conceptRun{discovery: discovery}}
	for _, item := range discovery.items {
		if !asksBuildSteps(item, &prompt) {
			continue
		}

		verdict, err := execution.patternBindings(item, checks.Input{})
		if err != nil {
			return err
		}

		result := validate.CheckResult(prompt, verdict)
		slog.Info("Check answered", "check", patternBindingsID, "subjectID", item.subjectID,
			"promptIndex", prompt.Index, "answer", result.ActualAnswer, "context", result.Context)

		err = validate.SaveCheckVerdict(deps.TmpDir, item.subjectID, result)
		if err != nil {
			return err
		}
	}

	return nil
}
