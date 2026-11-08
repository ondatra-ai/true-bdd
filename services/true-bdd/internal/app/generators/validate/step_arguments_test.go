package validate_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

const argumentSection = `step_arguments:
  - name: regular expression
    read: |
      The complete tail, including "quotes" and {{CWD}}.
      No token expansion here.
    applies: stdout and stderr
    escaping: none
`

func declareArgumentSection(t *testing.T) {
	t.Helper()

	body, err := disk.Read(argumentArchitecturePath)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write(argumentArchitecturePath, append(body, []byte(argumentSection)...), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}

func argumentProtocol(t *testing.T, evaluator *validate.ChecklistEvaluator, kind string,
) (any, checklist.PromptWithContext) {
	t.Helper()

	item := &template.ScenarioCheckData{ID: linkTestConcept, Service: linkTestService,
		CheckedSteps:      []catalogue.Step{{Block: catalogue.BlockWhen, Text: linkTestStepText}},
		ConceptLinkPassed: true, ConceptLinks: []template.ConceptLink{{Block: catalogue.BlockWhen, Concept: "square"}}}

	prompt := checklist.PromptWithContext{Index: 1,
 Prompt: checklist.Prompt{Question: "Judge", With: map[string]any{conceptsParameter: kind}}}
	switch kind {
	case argumentProposeJudgement:
		bindHalve(t)

		return proposalSubject(), proposalPrompt()
	case argumentBindingsJudgement, argumentSharedWordsJudgement:
		err := disk.Write(argumentArchitecturePath, []byte(boundArchitecture), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}

		evaluator.StartWalk()
		evaluator.ShareSteps(item.ID, item.Service, item.ConceptLinks[0], []string{linkTestStepText})

		if kind == argumentSharedWordsJudgement {
			prompt.Prompt.With["concepts"] = argumentWordsJudgement
		}
	}

	return item, prompt
}

func TestConceptProtocolsReceiveCompleteStepArguments(t *testing.T) {
	for _, kind := range argumentProtocolKinds() {
		t.Run(kind, func(t *testing.T) {
			evaluator, reply, _ := conceptRequestEvaluator(t)
			subject, prompt := argumentProtocol(t, evaluator, kind)
			declareArgumentSection(t)
			_, _ = evaluator.EvaluateOne(t.Context(), subject, "argument-item", prompt, t.TempDir())

			var payload struct {
				StepArguments string `json:"step_arguments"`
			}

			err := json.Unmarshal([]byte(reply.user), &payload)
			if err != nil {
				t.Fatal(err)
			}

			if payload.StepArguments != argumentSection {
				t.Fatalf("%s lost complete argument section: %q", kind, payload.StepArguments)
			}
		})
	}
}

func TestConceptProtocolsWithoutStepArgumentsKeepTheirBytes(t *testing.T) {
	for _, kind := range argumentProtocolKinds() {
		t.Run(kind, func(t *testing.T) {
			evaluator, reply, _ := conceptRequestEvaluator(t)
			subject, prompt := argumentProtocol(t, evaluator, kind)
			_, _ = evaluator.EvaluateOne(t.Context(), subject, "argument-item", prompt, t.TempDir())

			baselines := map[string]string{
				linkJudgement:         "3228606d0ff9f227f3d24872972aab7ca656cf2b87265b001e0ea1338ec97a8a",
				argumentGroupJudgement:        "b699e04c02a0647430ca16ae5dc48e74cedd2d10d09a80f4285add3652909d46",
				argumentWordsJudgement:        "ddbdbb85f00948c37da255cf1b9ee0bab26e704b185ed300b12f1925dd1fda5e",
				argumentSharedWordsJudgement: "9e635130a1c6a671ae8647a9cf0012206fc6351c357e7c94a02323bba99090c8",
				argumentBindingsJudgement:     "7af8fc5c7f30d99f29cac5b4b30862a8af899bd9f3e0f56670e1486e72d14283",
				argumentProposeJudgement:      "3062a8c344fc5a13ee94c877173154b58a80bc3c03283f35627c235f43279aaa",
			}
			if hash := fmt.Sprintf("%x", sha256.Sum256([]byte(reply.system+reply.user))); hash != baselines[kind] {
				t.Fatalf("%s absent-section bytes changed: %s", kind, hash)
			}
		})
	}
}

const argumentSharedWordsJudgement = "shared-words"

func argumentProtocolKinds() []string {
 return []string{linkJudgement, argumentGroupJudgement, argumentWordsJudgement, argumentSharedWordsJudgement,
  argumentBindingsJudgement, argumentProposeJudgement}
}
