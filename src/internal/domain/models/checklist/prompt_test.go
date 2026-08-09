package checklist_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/src/internal/domain/models/checklist"
)

const (
	tierXHigh = "xhigh"
	tierHigh  = "high"
	tierCoder = "coder"
)

// The checklist YAML is hand-decoded (Prompt.UnmarshalYAML), so the new
// tier keys need a decode test — an unrecognised key fails open there,
// silently ignoring the override.
func TestChecklistDecodesEngineBlockAndPromptTiers(t *testing.T) {
	t.Parallel()

	const source = `
version: "1.0"
config:
  max_apply_attempts: 5
engine:
  prompt_model: xhigh
  fix_model: high
  apply_model: coder
sections:
  - id: test-passes
    name: "Test Passes"
    validation_prompts:
      - Q: "inherits every tier"
        rationale: "why"
      - Q: "overrides the validation tier only"
        model: coder
      - Q: "overrides all three"
        model: high
        fix_model: coder
        apply_model: xhigh
`

	var doc checklist.Checklist

	err := yaml.Unmarshal([]byte(source), &doc)
	if err != nil {
		t.Fatalf("unmarshal checklist: %v", err)
	}

	if doc.Engine == nil {
		t.Fatal("checklist engine block was not decoded")
	}

	if doc.Engine.PromptModel != tierXHigh || doc.Engine.FixModel != tierHigh ||
		doc.Engine.ApplyModel != tierCoder {
		t.Fatalf("engine block = %+v", *doc.Engine)
	}

	prompts := doc.Sections[0].ValidationPrompts
	if len(prompts) != 3 {
		t.Fatalf("decoded %d prompts, want 3", len(prompts))
	}

	if prompts[1].Model != tierCoder {
		t.Errorf("prompt[1].Model = %q, want coder", prompts[1].Model)
	}

	if prompts[2].FixModel != tierCoder || prompts[2].ApplyModel != tierXHigh {
		t.Errorf("prompt[2] tiers = %q / %q", prompts[2].FixModel, prompts[2].ApplyModel)
	}
}

func TestEffectiveTierResolutionOrder(t *testing.T) {
	t.Parallel()

	checklistDefaults := &checklist.EngineBlock{
		PromptModel: tierXHigh,
		FixModel:    tierHigh,
		ApplyModel:  tierCoder,
	}

	tests := []struct {
		name                          string
		promptWithContext             checklist.PromptWithContext
		wantModel, wantFix, wantApply string
	}{
		{
			name: "checklist defaults apply when the prompt is silent",
			promptWithContext: checklist.PromptWithContext{
				DefaultModels: checklistDefaults,
			},
			wantModel: tierXHigh, wantFix: tierHigh, wantApply: tierCoder,
		},
		{
			name: "prompt overrides shadow the checklist per role",
			promptWithContext: checklist.PromptWithContext{
				DefaultModels: checklistDefaults,
				Prompt:        checklist.Prompt{Model: tierCoder, ApplyModel: tierHigh},
			},
			wantModel: tierCoder, wantFix: tierHigh, wantApply: tierHigh,
		},
		{
			// Nothing anywhere: every tier is empty, which the registry
			// reads as engine.default_model.
			name:      "no checklist block and no prompt override",
			wantModel: "", wantFix: "", wantApply: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			promptCtx := testCase.promptWithContext

			if got := promptCtx.EffectiveModelTier(); got != testCase.wantModel {
				t.Errorf("EffectiveModelTier() = %q, want %q", got, testCase.wantModel)
			}

			if got := promptCtx.EffectiveFixTier(); got != testCase.wantFix {
				t.Errorf("EffectiveFixTier() = %q, want %q", got, testCase.wantFix)
			}

			if got := promptCtx.EffectiveApplyTier(); got != testCase.wantApply {
				t.Errorf("EffectiveApplyTier() = %q, want %q", got, testCase.wantApply)
			}
		})
	}
}
