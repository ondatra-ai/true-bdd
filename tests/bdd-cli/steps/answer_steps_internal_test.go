package steps

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// A refine answers "2", types its feedback, ends it with an empty line and
// applies with "1" (#168): the pieces reach stdin in turn, trimmed, and no more.
func TestAnswersLineByLineFeedsEachPieceInTurn(t *testing.T) {
	t.Parallel()

	state := &State{T: t, Fixture: &runner.Fixture{}}

	err := answerLineByLine(state, []string{"Quality Engineer", "2 / Name the slot limit / / 1"})
	if err != nil {
		t.Fatalf("answers line by line: %v", err)
	}

	want := "2\nName the slot limit\n\n1\n"
	if got := string(state.Fixture.Stdin); got != want {
		t.Fatalf("stdin = %q, want %q", got, want)
	}
}

// Like every Given that feeds the run, it needs the tree the run starts from.
func TestAnswersLineByLineNeedsATree(t *testing.T) {
	t.Parallel()

	err := answerLineByLine(&State{T: t}, []string{"Quality Engineer", "1"})
	if err == nil {
		t.Fatal("answers line by line without a tree: want an error")
	}
}
