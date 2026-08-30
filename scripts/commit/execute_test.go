package commit_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/commit"
)

// The property task-handle depends on: a refused run returns instead of taking
// the whole process with it.
func TestGuardReturnsAStopAsAnError(t *testing.T) {
	t.Parallel()

	err := commit.Guard(func() { commit.PanicStop("no pull request open") })
	if err == nil {
		t.Fatal("a stop returned no error; an importer would carry on past a refusal")
	}

	var stopped *commit.StopError
	if !errors.As(err, &stopped) {
		t.Fatalf("got %T, want *commit.StopError", err)
	}

	if stopped.Message != "no pull request open" {
		t.Errorf("message = %q, want the diagnosis verbatim", stopped.Message)
	}
}

func TestGuardReturnsNilWhenNothingStops(t *testing.T) {
	t.Parallel()

	err := commit.Guard(func() {})
	if err != nil {
		t.Errorf("guard() = %v, want nil", err)
	}
}

// A real bug must not become a polite error.
func TestGuardRepanicsAnythingThatIsNotAStop(t *testing.T) {
	t.Parallel()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("a nil-map panic was swallowed into an error")
		}

		if _, ok := recovered.(string); !ok {
			t.Errorf("re-panicked %T, want the original value", recovered)
		}
	}()

	_ = commit.Guard(func() { panic("index out of range") })
}

func TestIsGatesRedMatchesOnlyTheOneRecoverableStop(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		err  error
		want bool
	}{
		"the gates":  {&commit.StopError{Message: commit.GatesRedPrefix + "\n  lint"}, true},
		"a push":     {&commit.StopError{Message: "gh push failed (1)"}, false},
		"not a stop": {errors.New("some other failure"), false},
		"nil":        {nil, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := commit.IsGatesRed(testCase.err); got != testCase.want {
				t.Errorf("IsGatesRed(%v) = %v, want %v", testCase.err, got, testCase.want)
			}
		})
	}
}

// The prefix is the contract task-handle recovers on; a reworded dief would
// silently turn every red gate into a halt.
func TestGatesRedPrefixIsWhatDiefEmits(t *testing.T) {
	t.Parallel()

	if !strings.HasPrefix(commit.GatesRedPrefix, "the gates are red") {
		t.Errorf("GatesRedPrefix = %q, want the gate diagnosis", commit.GatesRedPrefix)
	}
}
