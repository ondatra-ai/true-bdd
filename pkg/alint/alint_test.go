package alint_test

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/alint"
	"github.com/ondatra-ai/true-bdd/pkg/console"
)

func TestRequestReadsTheEnvironmentAndArgv(t *testing.T) {
	t.Setenv(alint.PathVar, "scripts/lint/dispatch.go")
	t.Setenv(alint.RuleVar, "go-lint-scoped")
	t.Setenv(alint.LevelVar, "error")

	got := alint.Request([]string{"go-package", alint.FixFlag, "scripts/lint"})

	if !got.Fix {
		t.Error("FixFlag in the argv must set AlintLintParams.Fix")
	}

	if !slices.Equal(got.Args, []string{"go-package", "scripts/lint"}) {
		t.Errorf("args: got %q, want the argv without %s", got.Args, alint.FixFlag)
	}

	if got.Path != "scripts/lint/dispatch.go" || got.RuleID != "go-lint-scoped" {
		t.Errorf("request: got %+v, want the environment alint set", got)
	}
}

// No FixFlag is a checking run, and no ALINT_PATH is the whole tree — which is
// how the gate table's bare `lint <gate>` reaches the same closure.
func TestRequestWithoutFixOrPath(t *testing.T) {
	t.Setenv(alint.PathVar, "")

	got := alint.Request([]string{"markdown"})

	if got.Fix || got.Path != "" {
		t.Errorf("request: got %+v, want neither a fix nor a path", got)
	}
}

// A pass says nothing at all: alint reads a silent exit 0 as clean.
func TestAnswerIsSilentOnAPass(t *testing.T) {
	out := &bytes.Buffer{}
	console.SetDefault(console.New(out))

	err := alint.Answer(nil, func(alint.AlintLintParams, *slog.Logger) error { return nil })
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}

	if out.String() != "" {
		t.Errorf("a pass must write nothing, got %q", out)
	}
}

// The finding reaches the console this package owns — alint captures the
// child's output as the violation's message — and is the caller's verdict.
func TestAnswerWritesAndReturnsTheFinding(t *testing.T) {
	want := errors.New("a.go:4:2: declared and not used: x")
	out := &bytes.Buffer{}
	console.SetDefault(console.New(out))

	err := alint.Answer(nil, func(alint.AlintLintParams, *slog.Logger) error { return want })

	if !errors.Is(err, want) {
		t.Errorf("verdict: got %v, want %v", err, want)
	}

	if !bytes.Contains(out.Bytes(), []byte(want.Error())) {
		t.Errorf("the finding must reach the console, got %q", out)
	}
}
