package checklist_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
)

func TestASharedConceptJudgementIsAnOptInSwitchOfLinkBindingsAndWords(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		with   string
		valid  bool
		shares bool
		waits  bool
		batch  int
	}{
		{"{concepts: link}", true, false, false, 0},
		{"{concepts: link, share: true}", true, true, false, 10},
		{"{concepts: link, share: true, batch: 4}", true, true, false, 4},
		{"{concepts: bindings, share: true}", true, true, true, 0},
		{"{concepts: words, share: true}", true, true, false, 0},
		{"{concepts: words, share: false}", true, false, false, 0},
		{"{concepts: words, share: yes please}", false, false, false, 0},
		{"{concepts: group, share: true}", false, false, false, 0},
		{"{concepts: bindings, share: true, batch: 4}", false, false, false, 0},
		{"{concepts: link, share: true, batch: 0}", false, false, false, 0},
		{"{concepts: link, batch: 4}", false, false, false, 0},
	} {
		var prompt checklist.Prompt

		err := yaml.Unmarshal([]byte("Q: Judge.\nwith: "+test.with+"\n"), &prompt)
		if err != nil {
			t.Fatal(err)
		}

		if (prompt.ShapeError() == nil) != test.valid {
			t.Errorf("%s: shape error %v", test.with, prompt.ShapeError())
		}

		if test.valid && (prompt.SharesJudgement() != test.shares || prompt.WaitsForLinks() != test.waits ||
			prompt.SharedBatch() != test.batch) {
			t.Errorf("%s: shares=%v waits=%v batch=%d", test.with, prompt.SharesJudgement(), prompt.WaitsForLinks(),
				prompt.SharedBatch())
		}
	}
}
