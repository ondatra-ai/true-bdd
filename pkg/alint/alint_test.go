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

func TestAlintLintBuildsParams(t *testing.T) {
	t.Setenv(alint.PathVar, "scripts/lint/dispatch.go")
	t.Setenv(alint.RuleVar, "go-lint-scoped")
	t.Setenv(alint.LevelVar, "error")

	var got alint.AlintLintParams

	err := run(t, []string{"go-package", alint.FixFlag, "scripts/lint"},
		func(req alint.AlintLintParams, _ *slog.Logger) error {
			got = req

			return nil
		})
	if err != nil {
		t.Fatalf("AlintLint: %v", err)
	}

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

func TestAlintLintWithoutFixFlag(t *testing.T) {
	t.Setenv(alint.PathVar, "README.md")

	var got alint.AlintLintParams

	_ = run(t, []string{"markdown"}, func(req alint.AlintLintParams, _ *slog.Logger) error {
		got = req

		return nil
	})

	if got.Fix {
		t.Error("no FixFlag means a checking run, which must not rewrite")
	}
}

// A pass says nothing at all: alint reads a silent exit 0 as clean.
func TestAlintLintIsSilentOnAPass(t *testing.T) {
	t.Setenv(alint.PathVar, "a.go")

	out := &bytes.Buffer{}
	console.SetDefault(console.New(out))

	_ = alint.AlintLint(nil, func(alint.AlintLintParams, *slog.Logger) error { return nil })

	if out.String() != "" {
		t.Errorf("a pass must write nothing, got %q", out)
	}
}

// The finding reaches the console this package owns — alint captures the
// child's output as the violation's message — and is the caller's verdict.
func TestAlintLintWritesAndReturnsTheFinding(t *testing.T) {
	t.Setenv(alint.PathVar, "a.go")

	want := errors.New("a.go:4:2: declared and not used: x")
	out := &bytes.Buffer{}
	console.SetDefault(console.New(out))

	err := alint.AlintLint(nil, func(alint.AlintLintParams, *slog.Logger) error { return want })

	if !errors.Is(err, want) {
		t.Errorf("verdict: got %v, want %v", err, want)
	}

	if !bytes.Contains(out.Bytes(), []byte(want.Error())) {
		t.Errorf("the finding must reach the console, got %q", out)
	}
}

// run rebinds the Console so a gate's findings never reach the real stdout.
func run(t *testing.T, args []string, gate alint.Gate) error {
	t.Helper()

	console.SetDefault(console.New(&bytes.Buffer{}))

	return alint.AlintLint(args, gate)
}
