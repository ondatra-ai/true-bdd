package validate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// StartWalk begins a scen check walk: each binding and each linked concept's words are judged once in it (#264).
func (e *ChecklistEvaluator) StartWalk() {
	e.sharing.start()
}

// shares reports a Scenario's cell in a started walk, which answers from the walk's shared judgements;
// build steps' suites and us refine's Story are judged outside one, in their own turn.
func (e *ChecklistEvaluator) shares(subject any) bool {
	_, scenario := subject.(*template.ScenarioCheckData)

	return scenario && e.sharing.active()
}

// The judgement and concept attributes a shared turn's log record names it by.
const (
	sharedBindings    = "bindings"
	sharedConceptAttr = "concept"
)

func sharedKey(prompt checklist.PromptWithContext, kind, key string) string {
	return fmt.Sprintf("%02d %s %s", prompt.Index, kind, key)
}

// sharedTurn is one shared judgement's turn, its artifacts named for the key, not the Scenario that asked first;
// attrs name the judgement and concept it answers.
func (e *ChecklistEvaluator) sharedTurn(ctx context.Context, prompt checklist.PromptWithContext, key string,
	attrs []any, payload any, schema string,
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

	slog.Info("Shared concept judgement answered",
		append(append([]any{"promptIndex", prompt.Index}, attrs...), "askedBy", e.subjectID)...)

	return response, nil
}
