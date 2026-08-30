package hooks_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/claude/hooks"
)

func TestPostToolUsePassesTheEventThrough(t *testing.T) {
	t.Parallel()

	var got hooks.PostToolUseParams

	out := run(t, `{"tool_name":"Edit","tool_input":{"file_path":"/repo/a.go"}}`,
		func(params hooks.PostToolUseParams, _ *slog.Logger) error {
			got = params

			return nil
		})

	if got.ToolName != "Edit" || got.FilePath != "/repo/a.go" {
		t.Errorf("params: got %+v, want the event's tool and path", got)
	}

	// Nothing found is nothing written: a verdict Claude Code did not need.
	if out != "" {
		t.Errorf("a nil error must write nothing, got %q", out)
	}
}

func TestPostToolUseBlocksWithTheError(t *testing.T) {
	t.Parallel()

	out := run(t, `{"tool_name":"Write","tool_input":{"file_path":"a.go"}}`,
		func(_ hooks.PostToolUseParams, _ *slog.Logger) error {
			return errors.New("LINT FAILED on a.go")
		})

	var decoded struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}

	err := json.Unmarshal([]byte(out), &decoded)
	if err != nil {
		t.Fatalf("the verdict must be JSON: %v (%q)", err, out)
	}

	// `reason` is discarded unless `decision` is "block", so both are pinned.
	if decoded.Decision != "block" || decoded.Reason != "LINT FAILED on a.go" {
		t.Errorf("verdict: got %+v, want a block carrying the error", decoded)
	}
}

// A hook that cannot read its input has found nothing, which is not a finding.
func TestPostToolUseIsSilentOnGarbage(t *testing.T) {
	t.Parallel()

	called := false

	out := run(t, "not json at all", func(hooks.PostToolUseParams, *slog.Logger) error {
		called = true

		return errors.New("must not run")
	})

	if called {
		t.Error("an unreadable payload must not reach the gate")
	}

	if out != "" {
		t.Errorf("an unreadable payload must write nothing, got %q", out)
	}
}

// An event naming no file still reaches the gate; whether that is judgeable
// is the gate's call, not the harness's.
func TestPostToolUseForwardsAnEmptyPath(t *testing.T) {
	t.Parallel()

	seen := false

	run(t, `{"tool_name":"Bash","tool_input":{"command":"ls"}}`,
		func(params hooks.PostToolUseParams, _ *slog.Logger) error {
			seen = params.FilePath == "" && params.ToolName == "Bash"

			return nil
		})

	if !seen {
		t.Error("a tool that wrote no file must still reach the gate")
	}
}

func run(t *testing.T, payload string, answer hooks.PostToolUseFunc) string {
	t.Helper()

	out := &bytes.Buffer{}

	err := hooks.PostToolUse(strings.NewReader(payload), out, answer)
	if err != nil {
		t.Fatalf("PostToolUse: %v", err)
	}

	return out.String()
}
