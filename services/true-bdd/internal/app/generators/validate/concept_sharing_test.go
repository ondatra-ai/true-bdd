package validate_test

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

const sharedOverlapWait = 2 * time.Second

const (
	sharedBothID  = "scenario-both"
	resultConcept = "result"
	displayedStep = "36 is displayed"
	squaredStep   = "6 is squared"
)

type sharedTurn struct {
	user string
	keys []string
}

// sharedReply answers every key a turn's schema requires, holding the first turns until they overlap.
type sharedReply struct {
	mutex    sync.Mutex
	turns    []sharedTurn
	answer   func(key, user string) string
	together int
	release  chan struct{}
	overlap  bool
}

func newSharedReply(together int, answer func(key, user string) string) *sharedReply {
	return &sharedReply{answer: answer, together: together, release: make(chan struct{})}
}

func (reply *sharedReply) ExecutePromptWithSystem(ctx context.Context, _ provider.Role, _, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, schema string, _ ...provider.TurnOptions,
) (string, error) {
	top, keys := schemaKeys(schema)

	reply.mutex.Lock()
	reply.turns = append(reply.turns, sharedTurn{user: user, keys: keys})
	held := len(reply.turns) <= reply.together

	if len(reply.turns) == reply.together {
		close(reply.release)
	}
	reply.mutex.Unlock()

	if held {
		select {
		case <-reply.release:
			reply.mutex.Lock()
			reply.overlap = true
			reply.mutex.Unlock()
		case <-ctx.Done():
		case <-time.After(sharedOverlapWait):
		}
	}

	answers := make(map[string]json.RawMessage, len(keys))
	for _, key := range keys {
		answers[key] = json.RawMessage(reply.answer(key, user))
	}

	encoded, err := json.Marshal(map[string]any{top: answers})

	return string(encoded), err
}

func (reply *sharedReply) judged() []sharedTurn {
	reply.mutex.Lock()
	defer reply.mutex.Unlock()

	return slices.Clone(reply.turns)
}

func schemaKeys(schema string) (string, []string) {
	var contract struct {
		Properties map[string]struct {
			Properties map[string]any `json:"properties"`
		} `json:"properties"`
	}

	if json.Unmarshal([]byte(schema), &contract) != nil || len(contract.Properties) != 1 {
		return "", nil
	}

	for top, inner := range contract.Properties {
		return top, slices.Sorted(maps.Keys(inner.Properties))
	}

	return "", nil
}

func evaluateTogether(t *testing.T, evaluator *validate.ChecklistEvaluator, prompt checklist.PromptWithContext,
	subjects ...*template.ScenarioCheckData,
) map[string]checklist.ValidationResult {
	t.Helper()

	var (
		mutex   sync.Mutex
		group   sync.WaitGroup
		failure error
	)

	results := make(map[string]checklist.ValidationResult, len(subjects))

	for _, subject := range subjects {
		group.Go(func() {
			result, err := evaluator.EvaluateOne(t.Context(), subject, subject.ID, prompt, t.TempDir())

			mutex.Lock()
			defer mutex.Unlock()

			results[subject.ID] = result

			if err != nil {
				failure = err
			}
		})
	}

	group.Wait()

	if failure != nil {
		t.Fatal(failure)
	}

	return results
}

func conceptPrompt(kind string) checklist.PromptWithContext {
	return checklist.PromptWithContext{Index: 3, SectionID: "concept", Prompt: checklist.Prompt{
		Question: "Judge " + kind, With: map[string]any{conceptsParameter: kind},
	}}
}

func wordAnswer(key, _ string) string {
	if key == "then:result" {
		return `{"undeclared_words":["settlement"],"reason":"A domain word."}`
	}

	return `{"undeclared_words":[],"reason":"Plain English."}`
}

func TestWordsJudgeEachLinkedConceptOncePerWalk(t *testing.T) {
	reply := newSharedReply(2, wordAnswer)
	evaluator, _ := conceptRequestEvaluatorWith(t, reply)
	square := template.ConceptLink{Concept: linkTestConcept, Block: catalogue.BlockWhen}
	outcome := template.ConceptLink{Concept: resultConcept, Block: catalogue.BlockThen}
	both := &template.ScenarioCheckData{ID: sharedBothID, ConceptLinks: []template.ConceptLink{square, outcome},
		ConceptLinkPassed: true}
	one := &template.ScenarioCheckData{ID: "scenario-one", ConceptLinks: []template.ConceptLink{square},
		ConceptLinkPassed: true}
	unlinked := &template.ScenarioCheckData{ID: "scenario-unlinked"}

	evaluator.StartWalk()
	results := evaluateTogether(t, evaluator, conceptPrompt("words"), both, one, unlinked)

	turns := reply.judged()
	if len(turns) != 2 || !reply.overlap {
		t.Fatalf("want one overlapping turn per linked concept, got %+v (overlap %v)", turns, reply.overlap)
	}

	for _, turn := range turns {
		if len(turn.keys) != 1 || strings.Contains(turn.user, "scenario-") {
			t.Fatalf("a shared word turn judges one concept and no Scenario: %+v", turn)
		}
	}

	assertContext(t, results[sharedBothID], checklist.StatusFail, "then:result: A domain word.",
		"then:result: undeclared domain word settlement", "when:square: Plain English.",
		"when:square: declared terms and plain English only")
	assertContext(t, results["scenario-one"], checklist.StatusPass, "when:square: Plain English.",
		"when:square: declared terms and plain English only")
	assertContext(t, results["scenario-unlinked"], checklist.StatusFail,
		"word judgement requires this walk's successful link")

	evaluator.StartWalk()
	evaluateTogether(t, evaluator, conceptPrompt("words"), one)

	if len(reply.judged()) != 3 {
		t.Fatalf("a new walk judges its concepts again: %+v", reply.judged())
	}
}

func TestAWordJudgementOutsideAWalkJudgesItsSubjectAlone(t *testing.T) {
	reply := newSharedReply(0, wordAnswer)
	evaluator, _ := conceptRequestEvaluatorWith(t, reply)
	both := &template.ScenarioCheckData{ID: sharedBothID, ConceptLinkPassed: true, ConceptLinks: []template.ConceptLink{
		{Concept: linkTestConcept, Block: catalogue.BlockWhen}, {Concept: resultConcept, Block: catalogue.BlockThen},
	}}

	evaluateTogether(t, evaluator, conceptPrompt("words"), both)

	turns := reply.judged()
	if len(turns) != 1 || len(turns[0].keys) != 2 || !strings.Contains(turns[0].user, sharedBothID) {
		t.Fatalf("without a walk the Scenario's own turn judges: %+v", turns)
	}
}

func assertContext(t *testing.T, result checklist.ValidationResult, status checklist.Status, context ...string) {
	t.Helper()

	if result.Status != status || !slices.Equal(result.Context, context) {
		t.Errorf("want %s %q, got %s %q", status, context, result.Status, result.Context)
	}
}
