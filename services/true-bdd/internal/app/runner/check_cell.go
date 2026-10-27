package runner

import (
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
)

// answerCheck answers a `D:` Cell with the built-in check its prompt
// names, reading the documents the prompt cites; it never dispatches a turn.
func answerCheck[I any](
	spec Spec[I],
	item I,
	subjectID string,
	query *checklistmodels.PromptWithContext,
) (checklistmodels.ValidationResult, error) {
	prompt := *query
	name := prompt.Prompt.Check

	docPaths, err := spec.DocResolver.ResolveAll(prompt.GetEffectiveDocs())
	if err != nil {
		return checklistmodels.ValidationResult{}, fmt.Errorf("check %s: %w", name, err)
	}

	verdict, err := spec.Checks[name].Run(item, checks.Input{Docs: docPaths})
	if err != nil {
		// Logged here: the walk stops on it, and cobra prints the error to
		// stderr, which no `stdout matches` step reads.
		slog.Error("Check could not answer",
			"check", name,
			"subjectID", subjectID,
			"promptIndex", query.Index,
			"error", err,
		)

		return checklistmodels.ValidationResult{}, fmt.Errorf("check %s: %w", name, err)
	}

	result := validate.CheckResult(prompt, verdict)

	slog.Info("Check answered",
		"check", name,
		"subjectID", subjectID,
		"promptIndex", query.Index,
		"answer", result.ActualAnswer,
		"context", result.Context,
	)

	err = validate.SaveCheckVerdict(spec.TmpDir, subjectID, result)
	if err != nil {
		return checklistmodels.ValidationResult{}, fmt.Errorf("check %s: %w", name, err)
	}

	return result, nil
}

// recordSkip reports a `Q:` Cell a failed check spared as SKIP, so the
// report still shows every prompt the item was held to.
func recordSkip[I any](spec Spec[I], builder *reportBuilder) func(item I, query *checklistmodels.PromptWithContext) {
	return func(item I, query *checklistmodels.PromptWithContext) {
		subjectID, subjectTitle := spec.GetSubject(item)

		slog.Info("Prompt skipped after a failed check",
			"subjectID", subjectID,
			"promptIndex", query.Index,
		)

		builder.Add(subjectID, subjectTitle, validate.SkippedResult(*query))
	}
}

// isCheckQuery marks a `D:` Cell as the walker's gate: its failure skips
// the item's model Cells until a fix restarts the item.
func isCheckQuery(query *checklistmodels.PromptWithContext) bool {
	return query.Prompt.IsCheck()
}
