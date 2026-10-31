package runner

import (
	"reflect"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
)

func TestWalkLinksBeforeSketchBindingAndWordJudgements(t *testing.T) {
	t.Parallel()

	const conceptsKey = "concepts"

	prompts := []checklist.PromptWithContext{
		{Index: 1, Prompt: checklist.Prompt{Question: "Sketch", With: map[string]any{conceptsKey: "sketch"}}},
		{Index: 2, Prompt: checklist.Prompt{Question: "Bindings", With: map[string]any{conceptsKey: "bindings"}}},
		{Index: 3, Prompt: checklist.Prompt{Check: "public-interface"}},
		{Index: 4, Prompt: checklist.Prompt{Question: "Link", With: map[string]any{conceptsKey: "link"}}},
		{Index: 5, Prompt: checklist.Prompt{Check: "concept-bindings"}},
		{Index: 6, Prompt: checklist.Prompt{Question: "Words", With: map[string]any{conceptsKey: "words"}}},
	}

	indices := make([]int, 0, len(prompts))
	for _, prompt := range walkOrder(prompts) {
		indices = append(indices, prompt.Index)
	}

	if !reflect.DeepEqual(indices, []int{3, 5, 4, 1, 2, 6}) {
		t.Fatalf("want D gates, link, then other Q turns with original artifact indices; got %v", indices)
	}

	if prompts[0].Index != 1 || prompts[1].Index != 2 {
		t.Fatal("ordering mutated the file-order prompts")
	}
}
