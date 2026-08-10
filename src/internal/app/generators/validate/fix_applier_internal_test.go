package validate

import (
	"errors"
	"strings"
	"testing"

	pkgerrors "github.com/ondatra-ai/true-bdd/src/internal/pkg/errors"
)

// An applier turn can finish cleanly having written nothing — the
// guard denied it, or the only path offered was a forbidden one. The
// engine used to read that as a successful fix and re-walk forever, so
// the explicit `applied: false` has to surface as an error.
func TestCheckFixAppliedRejectsExplicitFalse(t *testing.T) {
	t.Parallel()

	content := `applied: false
target: services/frontend/*
summary: "Blocked by true-bdd guard: write root is tmp/, not services/."`

	err := checkFixApplied(content)
	if !errors.Is(err, pkgerrors.ErrFixNotApplied) {
		t.Fatalf("checkFixApplied() error = %v, want ErrFixNotApplied", err)
	}

	// The model's own summary is the diagnosis — losing it turns an
	// actionable failure into "something went wrong".
	if msg := err.Error(); !strings.Contains(msg, "services/frontend/*") ||
		!strings.Contains(msg, "Blocked by true-bdd guard") {
		t.Errorf("error message dropped target or summary: %q", msg)
	}
}

func TestCheckFixAppliedAcceptsSuccessAndAbsence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name: "explicit true",
			content: `applied: true
target: services/frontend/app/page.tsx
summary: "Created the home page."`,
		},
		{
			// us create / us refine emit a story body with no `applied:`
			// key at all. Absent must not read as false.
			name: "story body with no applied key",
			content: `id: "99.1"
title: Document Summary On Demand
as_a: Claude User`,
		},
		{
			// Several appliers return prose or a non-mapping body; that
			// is not this check's business.
			name:    "unparseable content",
			content: "I created the file at services/frontend/page.tsx.",
		},
		{
			name:    "fenced yaml",
			content: "```yaml\napplied: true\ntarget: services/x\n```",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := checkFixApplied(testCase.content)
			if err != nil {
				t.Fatalf("checkFixApplied() = %v, want nil", err)
			}
		})
	}
}

// The block a real run produced: the model put an explanatory `": "`
// inside an unquoted `target:` scalar, so the whole block is invalid
// YAML. Passing that through logged "Fix applied successfully" for an
// applier that had explicitly written nothing.
func TestCheckFixAppliedRejectsFalseInUnparseableBlock(t *testing.T) {
	t.Parallel()

	content := `applied: false
target: services/frontend/ (multiple new files: package.json, app/page.tsx)
summary: "Could not write any file — every tool was blocked."`

	// Precondition: this really is undecodable, otherwise the test
	// would pass through the normal path and prove nothing.
	if _, ok := parseApplyResult(content); ok {
		t.Fatal("fixture parsed as YAML; it no longer exercises the fallback")
	}

	err := checkFixApplied(content)
	if !errors.Is(err, pkgerrors.ErrFixNotApplied) {
		t.Fatalf("checkFixApplied() error = %v, want ErrFixNotApplied", err)
	}
}

// The fallback must stay as narrow as the parser it backs up: content
// that merely mentions the word must not be read as a verdict.
func TestCheckFixAppliedFallbackIgnoresProse(t *testing.T) {
	t.Parallel()

	tests := []string{
		"The fix was applied: false starts were needed.",
		"I applied: the change to services/api and it worked.",
		"applied: maybe",
	}

	for _, content := range tests {
		t.Run(content, func(t *testing.T) {
			t.Parallel()

			err := checkFixApplied(content)
			if err != nil {
				t.Fatalf("checkFixApplied() = %v, want nil", err)
			}
		})
	}
}

// A fenced `applied: false` must still be caught — models wrap the
// confirmation block in a code fence often enough that only stripping
// it on the success path would leave a hole.
func TestCheckFixAppliedRejectsFencedFalse(t *testing.T) {
	t.Parallel()

	content := "```yaml\napplied: false\ntarget: services/api\nsummary: \"denied\"\n```"

	err := checkFixApplied(content)
	if !errors.Is(err, pkgerrors.ErrFixNotApplied) {
		t.Fatalf("checkFixApplied() error = %v, want ErrFixNotApplied", err)
	}
}
