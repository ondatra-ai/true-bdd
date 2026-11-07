package testkit_test

import (
	"regexp"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit"
)

func TestCheckStdoutMatchesAcrossLinesInTheWholeStream(t *testing.T) {
	t.Parallel()

	stdout := "before\nFIX PROMPT GENERATED\nquestion\nREFINE FIX PROMPT\nafter\n"

	for _, pattern := range []string{
		`(?s)FIX PROMPT GENERATED.*REFINE FIX PROMPT`,
		`\Abefore\nFIX PROMPT GENERATED\nquestion\nREFINE FIX PROMPT\nafter\n\z`,
	} {
		err := testkit.CheckStdout(stdout, regexp.MustCompile(pattern))
		if err != nil {
			t.Errorf("whole-stream pattern %q: %v", pattern, err)
		}
	}

	err := testkit.CheckStdout(stdout, regexp.MustCompile(`(?s)REFINE FIX PROMPT.*FIX PROMPT GENERATED`))
	if err == nil {
		t.Fatal("a sequence absent from stdout matched")
	}
}
