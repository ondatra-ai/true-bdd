package merge_test

import (
	"errors"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/merge"
)

// The property task-handle depends on: a refused merge returns instead of
// taking the whole process with it.
func TestGuardReturnsAStopAsAnError(t *testing.T) {
	t.Parallel()

	err := merge.Guard(func() { merge.PanicStop("detached HEAD") })
	if err == nil {
		t.Fatal("a stop returned no error; an importer would carry on past a refusal")
	}

	var stopped *merge.StopError
	if !errors.As(err, &stopped) {
		t.Fatalf("got %T, want *merge.StopError", err)
	}

	if stopped.Message != "detached HEAD" {
		t.Errorf("message = %q, want the diagnosis verbatim", stopped.Message)
	}
}

func TestGuardReturnsNilWhenNothingStops(t *testing.T) {
	t.Parallel()

	err := merge.Guard(func() {})
	if err != nil {
		t.Errorf("guard() = %v, want nil", err)
	}
}

func TestGuardRepanicsAnythingThatIsNotAStop(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("a nil-map panic was swallowed into an error")
		}
	}()

	_ = merge.Guard(func() { panic("index out of range") })
}
