package validate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// StartWalk begins a walk of these subjects: a shared concept judgement answers each of its keys once in it (#264).
func (e *ChecklistEvaluator) StartWalk(subjects []any) {
	e.sharing.start(subjects)
}

// shares reports whether this cell takes its verdicts from the walk's shared judgements.
func (e *ChecklistEvaluator) shares(subject any, prompt checklist.PromptWithContext) bool {
	_, scenario := subject.(*template.ScenarioCheckData)

	return scenario && prompt.Prompt.SharesJudgement() && e.sharing.active()
}

func sharedKey(prompt checklist.PromptWithContext, kind, key string) string {
	return fmt.Sprintf("%02d %s %s", prompt.Index, kind, key)
}

// sharedTurn is one shared judgement's turn, its artifacts named for the key, not the Scenario that asked first.
func (e *ChecklistEvaluator) sharedTurn(ctx context.Context, prompt checklist.PromptWithContext, key string,
	payload any, schema string,
) (string, error) {
	local := *e
	local.subjectID = "shared-" + key

	if e.artifactLabel != "" {
		local.artifactLabel = fmt.Sprintf("validation-%02d-%x", prompt.Index, sha256.Sum256([]byte(key)))
	}

	response, err := local.conceptTurn(ctx, prompt, payload, schema)
	if err != nil {
		return "", err
	}

	slog.Info("Shared concept judgement answered", "promptIndex", prompt.Index, "key", key, "askedBy", e.subjectID)

	return response, nil
}
