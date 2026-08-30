package crush_test

import (
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/crush"
)

// The model MUST arrive as `-m`: crush silently ignores an unknown model
// pinned in config and falls back to global state, so a config pin would fail
// invisibly.
func TestArgsPassesTheModelAsAFlag(t *testing.T) {
	t.Parallel()

	args := crush.Turn{Model: "zhipu-coding/glm-5.2", WorkDir: "/repo"}.Args()

	want := []string{"crush", "run", "--quiet", "-m", "zhipu-coding/glm-5.2", "--cwd", "/repo"}
	if !slices.Equal(args, want) {
		t.Fatalf("Args() = %v, want %v", args, want)
	}
}

// crush parses a hook command as a shell line and FAILS OPEN on one it cannot
// run, so an unquoted quote in the path would silently kill the write gate.
func TestQuoteSurvivesAnEmbeddedQuote(t *testing.T) {
	t.Parallel()

	if got, want := crush.Quote("/o'brien/bin"), `'/o'\''brien/bin'`; got != want {
		t.Fatalf("Quote = %s, want %s", got, want)
	}
}
