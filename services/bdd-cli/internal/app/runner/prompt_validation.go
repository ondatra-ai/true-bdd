package runner

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/checks"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
)

// errPromptsNotWalkable is the canonical error returned by
// validatePrompts, kept free of regex metacharacters for `stdout matches`.
var errPromptsNotWalkable = errors.New("checklist has prompts the engine cannot walk")

// Why a well-shaped `D:` prompt still cannot be walked by this command.
var (
	errUnknownCheck      = errors.New("names no built-in check of")
	errUnknownCheckParam = errors.New("has no with: parameter")
	errCheckDocUnlisted  = errors.New("so its docs: must list")
)

// validatePrompts refuses a checklist holding a prompt nothing could
// answer as written, naming each by its position, before any turn is paid.
func validatePrompts[I any](
	command string,
	set checks.Set[I],
	prompts []checklistmodels.PromptWithContext,
) error {
	offenders := make([]string, 0)

	for idx := range prompts {
		err := promptProblem(command, set, &prompts[idx])
		if err == nil {
			continue
		}

		offenders = append(offenders, fmt.Sprintf("%s #%d %s",
			prompts[idx].GetFullSectionPath(), prompts[idx].Index, err))
	}

	if len(offenders) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s", errPromptsNotWalkable, strings.Join(offenders, "; "))
}

// promptProblem is why one prompt cannot be walked by this command: its
// shape, a check the command does not hold, a parameter the check lacks,
// or a document the check reads that the prompt does not cite.
func promptProblem[I any](command string, set checks.Set[I], promptCtx *checklistmodels.PromptWithContext) error {
	prompt := &promptCtx.Prompt

	err := prompt.ShapeError()
	if err != nil {
		return err
	}

	if !prompt.IsCheck() {
		return nil
	}

	check, known := set[strings.TrimSpace(prompt.Check)]
	if !known {
		return fmt.Errorf("%s %w %s, which has %s", prompt.Label(), errUnknownCheck, command, set.Describe())
	}

	keys := make([]string, 0, len(prompt.With))
	for key := range prompt.With {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	for _, key := range keys {
		if !slices.Contains(check.Params, key) {
			return fmt.Errorf("%s %w %s; it takes %s",
				prompt.Label(), errUnknownCheckParam, key, describeParams(check.Params))
		}
	}

	return uncitedDoc(promptCtx, check.Docs)
}

// uncitedDoc refuses a check prompt that does not cite a document its
// check reads: the check would find nothing there and pass unchecked.
func uncitedDoc(promptCtx *checklistmodels.PromptWithContext, reads []string) error {
	cited := promptCtx.GetEffectiveDocs()

	for _, key := range reads {
		if !slices.Contains(cited, key) {
			return fmt.Errorf("%s reads the %s document, %w %s",
				promptCtx.Prompt.Label(), key, errCheckDocUnlisted, key)
		}
	}

	return nil
}

// describeParams renders a check's parameter list for a refusal.
func describeParams(params []string) string {
	if len(params) == 0 {
		return "none"
	}

	return strings.Join(params, ", ")
}

// walkOrder puts every `D:` prompt before any `Q:` one, each kind in file
// order, so a failed check can spare its item the model turns.
func walkOrder(prompts []checklistmodels.PromptWithContext) []checklistmodels.PromptWithContext {
	ordered := make([]checklistmodels.PromptWithContext, 0, len(prompts))

	for _, wantCheck := range []bool{true, false} {
		for idx := range prompts {
			if prompts[idx].Prompt.IsCheck() == wantCheck {
				ordered = append(ordered, prompts[idx])
			}
		}
	}

	return ordered
}
