package runner

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
)

// errPromptsNotWalkable is the canonical error returned by
// validatePrompts, kept free of regex metacharacters for `stdout matches`.
var errPromptsNotWalkable = errors.New("checklist has prompts the engine cannot walk")

// errTiersUnresolved is validateModelTiers' error, likewise free of them.
var errTiersUnresolved = errors.New("checklist has prompts whose model tier engine.models cannot resolve")

// checkRefusalError is why a well-shaped `D:` prompt still cannot be walked by
// this command: one whole sentence naming the prompt.
type checkRefusalError string

// Error is the sentence.
func (r checkRefusalError) Error() string { return string(r) }

// unknownCheck refuses a prompt naming a check its command does not hold.
func unknownCheck(label, command, held string) error {
	return checkRefusalError(fmt.Sprintf("%s names no built-in check of %s, which has %s", label, command, held))
}

// unreadParam refuses a prompt passing its check a parameter it never reads.
func unreadParam(label, key, takes string) error {
	return checkRefusalError(fmt.Sprintf("%s has no with: parameter %s; it takes %s", label, key, takes))
}

// uncitedDocument refuses a prompt not citing a document its check reads.
func uncitedDocument(label, key string) error {
	return checkRefusalError(fmt.Sprintf("%s reads the %s document, so its docs: must list %s", label, key, key))
}

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
		return conceptPromptDocs(promptCtx)
	}

	check, known := set[prompt.Check]
	if !known {
		return unknownCheck(prompt.Label(), command, set.Describe())
	}

	keys := make([]string, 0, len(prompt.With))
	for key := range prompt.With {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	for _, key := range keys {
		if !slices.Contains(check.Params, key) {
			return unreadParam(prompt.Label(), key, describeParams(check.Params))
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
			return uncitedDocument(promptCtx.Prompt.Label(), key)
		}
	}

	return nil
}

// validateModelTiers refuses a checklist naming a tier some Cell's turn
// could not run on, naming each prompt and turn, before the first turn:
// a Cell resolving it mid-walk fails only after earlier Cells have spent theirs.
func validateModelTiers[I any](spec Spec[I], prompts []checklistmodels.PromptWithContext) error {
	offenders := make([]string, 0)

	for idx := range prompts {
		for _, err := range spec.Evaluator.UnresolvedTiers(&prompts[idx], spec.Fix) {
			offenders = append(offenders, fmt.Sprintf("%s #%d %s: %v",
				prompts[idx].GetFullSectionPath(), prompts[idx].Index, prompts[idx].Prompt.Label(), err))
		}
	}

	if len(offenders) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s", errTiersUnresolved, strings.Join(offenders, "; "))
}

// describeParams renders a check's parameter list for a refusal.
func describeParams(params []string) string {
	if len(params) == 0 {
		return "none"
	}

	return strings.Join(params, ", ")
}

const (
	phaseCheck = iota
	phaseLink
	phaseModel
)

// walkOrder answers D prompts, then concept links, then the other Q prompts.
// Each group keeps file order and every Cell keeps its original artifact index.
func walkOrder(prompts []checklistmodels.PromptWithContext) []checklistmodels.PromptWithContext {
	ordered := make([]checklistmodels.PromptWithContext, 0, len(prompts))
	for _, phase := range []int{phaseCheck, phaseLink, phaseModel} {
		for index := range prompts {
			if promptPhase(&prompts[index]) == phase {
				ordered = append(ordered, prompts[index])
			}
		}
	}

	return ordered
}

func promptPhase(prompt *checklistmodels.PromptWithContext) int {
	if prompt.Prompt.IsCheck() {
		return phaseCheck
	}

	if prompt.Prompt.With["concepts"] == linkJudgement {
		return phaseLink
	}

	return phaseModel
}

func conceptPromptDocs(prompt *checklistmodels.PromptWithContext) error {
	kind, configured := prompt.Prompt.With["concepts"]
	if !configured {
		return nil
	}

	required := []string{"product"}
	if kind == "bindings" || kind == "sketch" {
		required = append(required, "architecture_yaml")
	}

	return uncitedDoc(prompt, required)
}
