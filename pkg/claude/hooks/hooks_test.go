package hooks_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/claude/hooks"
	"github.com/ondatra-ai/true-bdd/pkg/console"
)

func TestPostToolUsePassesTheEventThrough(t *testing.T) {
	t.Parallel()

	var got hooks.PostToolUseParams

	reason, blocked := hooks.Judge(
		[]byte(`{"tool_name":"Edit","tool_input":{"file_path":"/repo/a.go"}}`),
		func(params hooks.PostToolUseParams, _ *slog.Logger) error {
			got = params

			return nil
		})

	if got.ToolName != "Edit" || got.FilePath != "/repo/a.go" {
		t.Errorf("params: got %+v, want the event's tool and path", got)
	}

	// Nothing found is nothing written: a verdict Claude Code did not need.
	if blocked || reason != "" {
		t.Errorf("a nil error must block nothing, got %q", reason)
	}
}

func TestPostToolUseBlocksWithTheError(t *testing.T) {
	t.Parallel()

	reason, blocked := hooks.Judge(
		[]byte(`{"tool_name":"Write","tool_input":{"file_path":"a.go"}}`),
		func(hooks.PostToolUseParams, *slog.Logger) error {
			return errors.New("LINT FAILED on a.go")
		})

	if !blocked || reason != "LINT FAILED on a.go" {
		t.Errorf("verdict: got blocked=%v reason=%q, want the error's message", blocked, reason)
	}
}

// A hook that cannot read its input has found nothing, which is not a finding.
func TestPostToolUseIsSilentOnGarbage(t *testing.T) {
	t.Parallel()

	called := false

	_, blocked := hooks.Judge([]byte("not json at all"),
		func(hooks.PostToolUseParams, *slog.Logger) error {
			called = true

			return errors.New("must not run")
		})

	if called {
		t.Error("an unreadable payload must not reach the gate")
	}

	if blocked {
		t.Error("an unreadable payload must block nothing")
	}
}

// An event naming no file still reaches the gate; whether that is judgeable
// is the gate's call, not the harness's.
func TestPostToolUseForwardsAnEmptyPath(t *testing.T) {
	t.Parallel()

	seen := false

	hooks.Judge([]byte(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`),
		func(params hooks.PostToolUseParams, _ *slog.Logger) error {
			seen = params.FilePath == "" && params.ToolName == "Bash"

			return nil
		})

	if !seen {
		t.Error("a tool that wrote no file must still reach the gate")
	}
}

// `reason` is discarded unless `decision` is "block", so both are pinned.
func TestBlockWritesTheVerdict(t *testing.T) {
	out := &bytes.Buffer{}
	console.SetDefault(console.New(out))

	err := hooks.Block("LINT FAILED on a.go")
	if err != nil {
		t.Fatalf("Block: %v", err)
	}

	var decoded struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}

	err = json.Unmarshal(out.Bytes(), &decoded)
	if err != nil {
		t.Fatalf("the verdict must be JSON: %v (%q)", err, out)
	}

	if decoded.Decision != "block" || decoded.Reason != "LINT FAILED on a.go" {
		t.Errorf("verdict: got %+v, want a block carrying the reason", decoded)
	}
}
