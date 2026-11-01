package validate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// StartWalk begins a walk, scen check's or build steps': each binding and each linked concept's words are judged
// once in it (#264, ADR-0037).
func (e *ChecklistEvaluator) StartWalk() {
	e.sharing.start()
}

// ShareSteps gives the walk's binding judgements a concept's steps for the service, as a passing link does;
// build steps shares each concept Item's grouped and resumed steps this way.
func (e *ChecklistEvaluator) ShareSteps(subjectID, service string, link template.ConceptLink, steps []string) {
	linked := make([]linkedStep, 0, len(steps))
	for _, text := range steps {
		linked = append(linked, linkedStep{service: service, key: link.Key(),
			step: conceptStep{Block: link.Block, Text: text}})
	}

	e.sharing.publish(subjectID, linked)
}

// shares reports a Scenario's or suite's cell in a started walk, which answers from the walk's shared
// judgements; us refine's Story is judged outside one, in its own turn.
func (e *ChecklistEvaluator) shares(subject any) bool {
	_, scenario := subject.(*template.ScenarioCheckData)

	return scenario && e.sharing.active()
}

// The concept attribute a shared turn's log record names it by.
const sharedConceptAttr = "concept"

func sharedKey(prompt checklist.PromptWithContext, kind checklist.ConceptJudgement, key string) string {
	return fmt.Sprintf("%02d %s %s", prompt.Index, kind, key)
}

// sharedJudgement is one judgement's keys, each answered once per walk: scope follows a key in its turn's name
// (a binding's service), attrs follow the concept in its log record, and request builds a key's payload and schema.
type sharedJudgement struct {
	judgement checklist.ConceptJudgement
	keys      []string
	scope     string
	attrs     []any
	request   func(key string) (any, string, error)
}

// judgeShared answers every key of the judgement, judging each one no cell has claimed yet; answers come back by key.
func (e *ChecklistEvaluator) judgeShared(ctx context.Context, prompt checklist.PromptWithContext,
	shared sharedJudgement,
) (map[string]string, error) {
	names := make([]string, 0, len(shared.keys))
	keys := make(map[string]string, len(shared.keys))

	for _, key := range shared.keys {
		name := sharedKey(prompt, shared.judgement, key+shared.scope)
		names = append(names, name)
		keys[name] = key
	}

	answers, err := e.sharing.resolve(ctx, names, func(name string) (string, error) {
		payload, schema, err := shared.request(keys[name])
		if err != nil {
			return "", err
		}

		attrs := append([]any{"judgement", string(shared.judgement), sharedConceptAttr, keys[name]}, shared.attrs...)

		return e.sharedTurn(ctx, prompt, name, attrs, payload, schema)
	})
	if err != nil {
		return nil, err
	}

	byKey := make(map[string]string, len(answers))
	for name, answer := range answers {
		byKey[keys[name]] = answer
	}

	return byKey, nil
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
