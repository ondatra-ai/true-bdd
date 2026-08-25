package history_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/history"
)

func TestBindRoundTrip(t *testing.T) {
	t.Parallel()

	hook := history.New(t.TempDir(), history.DefaultRole)

	if got := hook.Bound(); got != "" {
		t.Fatalf("a fresh Task is bound to %q, want none", got)
	}

	err := hook.Bind("  86cb8hjf7  ")
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}

	if got := hook.Bound(); got != "86cb8hjf7" {
		t.Fatalf("Bound() = %q, want the trimmed id", got)
	}

	err = hook.Unbind()
	if err != nil {
		t.Fatalf("Unbind: %v", err)
	}

	if got := hook.Bound(); got != "" {
		t.Fatalf("Bound() = %q after Unbind, want none", got)
	}
}

func TestBindRejectsEmpty(t *testing.T) {
	t.Parallel()

	err := history.New(t.TempDir(), history.DefaultRole).Bind("   ")
	if err == nil {
		t.Fatal("Bind(\"   \") succeeded, want a refusal")
	}
}

func TestUnbindIsIdempotent(t *testing.T) {
	t.Parallel()

	err := history.New(t.TempDir(), history.DefaultRole).Unbind()
	if err != nil {
		t.Fatalf("Unbind with nothing bound: %v", err)
	}
}

// The binding sits beside hook-state so that rolling one does not disturb the
// other: /task-start deletes hook-state and writes the binding in one call.
func TestBindingIsBesideHookState(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	err := history.New(repo, history.DefaultRole).Bind("86cb8hjf7")
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}

	_, err = os.Stat(filepath.Join(repo, "docs", "history", "bound-ticket"))
	if err != nil {
		t.Fatalf("binding not written beside hook-state: %v", err)
	}
}
