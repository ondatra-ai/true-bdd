package events_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/events"
)

// readEvents parses the JSONL event-channel file into decoded maps.
func readEvents(t *testing.T, path string) []map[string]any {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open events file: %v", err)
	}
	defer func() { _ = file.Close() }()

	var out []map[string]any

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event map[string]any

		err = json.Unmarshal([]byte(line), &event)
		if err != nil {
			t.Fatalf("unmarshal event line %q: %v", line, err)
		}

		out = append(out, event)
	}

	return out
}

// wantString asserts a string field.
func wantString(t *testing.T, event map[string]any, key, want string) {
	t.Helper()

	got, ok := event[key].(string)
	if !ok || got != want {
		t.Fatalf("event[%q] = %v, want %q", key, event[key], want)
	}
}

// wantOrdinal asserts a numeric ordinal (JSON numbers decode to float64).
func wantOrdinal(t *testing.T, event map[string]any, want float64) {
	t.Helper()

	got, ok := event["ordinal"].(float64)
	if !ok || got != want {
		t.Fatalf("event ordinal = %v, want %v", event["ordinal"], want)
	}
}

// payloadList reads a string-slice payload field.
func payloadList(t *testing.T, event map[string]any, key string) []string {
	t.Helper()

	payload, isMap := event["payload"].(map[string]any)
	if !isMap {
		t.Fatalf("event has no payload map: %v", event["payload"])
	}

	raw, isList := payload[key].([]any)
	if !isList {
		t.Fatalf("payload[%q] is not a list: %v", key, payload[key])
	}

	out := make([]string, 0, len(raw))
	for _, item := range raw {
		out = append(out, item.(string)) //nolint:forcetypeassert // test data is known
	}

	return out
}

// choiceActions is the apply/refine/exit fix-action vocabulary, shared so
// the literal lives once (goconst).
func choiceActions() []string {
	return []string{"apply", "refine", "exit"}
}

func TestEmitAllThreePromptShapes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(events.EventsFileEnv, path)

	emitter := events.NewEmitter()
	emitter.EmitChoicePrompt(choiceActions())
	emitter.EmitClarifyPrompt("Which duplicate wins?", []string{"first", "second"})
	emitter.EmitFreetextPrompt("Enter your feedback to improve the fix prompt.")

	lines := readEvents(t, path)
	if len(lines) != 3 {
		t.Fatalf("got %d events, want 3", len(lines))
	}

	wantString(t, lines[0], "type", "prompt")
	wantString(t, lines[0], "kind", "choice")
	wantString(t, lines[0], "prompt_id", "prompt-1")
	wantOrdinal(t, lines[0], 1)

	actions := payloadList(t, lines[0], "actions")
	if strings.Join(actions, ",") != strings.Join(choiceActions(), ",") {
		t.Fatalf("choice actions = %v", actions)
	}

	wantString(t, lines[1], "kind", "clarify")
	wantString(t, lines[1], "prompt_id", "prompt-2")
	wantOrdinal(t, lines[1], 2)

	options := payloadList(t, lines[1], "options")
	if strings.Join(options, ",") != "first,second" {
		t.Fatalf("clarify options = %v", options)
	}

	wantString(t, lines[2], "kind", "freetext")
	wantString(t, lines[2], "prompt_id", "prompt-3")
	wantOrdinal(t, lines[2], 3)
}

func TestEmitResultAfterFinalizationOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(events.EventsFileEnv, path)

	emitter := events.NewEmitter()
	emitter.EmitChoicePrompt(choiceActions())
	emitter.EmitResult("converged", false, "write story file: disk full")

	lines := readEvents(t, path)
	if len(lines) != 2 {
		t.Fatalf("got %d events, want 2", len(lines))
	}

	result := lines[1]
	wantString(t, result, "type", "result")
	wantString(t, result, "outcome", "converged")
	wantString(t, result, "detail", "write story file: disk full")
	wantOrdinal(t, result, 2) // strictly after the prompt's ordinal 1

	finalizationOK, ok := result["finalization_ok"].(bool)
	if !ok || finalizationOK {
		t.Fatalf("finalization_ok = %v, want false", result["finalization_ok"])
	}
}

func TestEmitResultFinalizationOKTrue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(events.EventsFileEnv, path)

	events.NewEmitter().EmitResult("ok", true, "")

	lines := readEvents(t, path)
	if len(lines) != 1 {
		t.Fatalf("got %d events, want 1", len(lines))
	}

	finalizationOK, ok := lines[0]["finalization_ok"].(bool)
	if !ok || !finalizationOK {
		t.Fatalf("finalization_ok = %v, want true", lines[0]["finalization_ok"])
	}
}

// TestSharedOrdinalAcrossProducers proves finding 7's shared ordinal
// source: the collector and the runner call NewEmitter() independently
// but share one instance, so the result ordinal follows the prompts.
func TestSharedOrdinalAcrossProducers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(events.EventsFileEnv, path)

	// The collector emits two prompts …
	collector := events.NewEmitter()
	collector.EmitChoicePrompt(choiceActions())
	collector.EmitFreetextPrompt("feedback")

	// … then the runner independently obtains an emitter and emits the
	// result. A fresh (non-shared) emitter would restart the ordinal at 1.
	runner := events.NewEmitter()
	runner.EmitResult("converged", true, "")

	lines := readEvents(t, path)
	if len(lines) != 3 {
		t.Fatalf("got %d events, want 3", len(lines))
	}

	wantOrdinal(t, lines[0], 1)
	wantOrdinal(t, lines[1], 2)
	wantString(t, lines[2], "type", "result")
	wantOrdinal(t, lines[2], 3) // the result FOLLOWS the two prompts
}

func TestEmitDisabledWithoutEnv(t *testing.T) {
	t.Setenv(events.EventsFileEnv, "")

	emitter := events.NewEmitter()
	if emitter.Enabled() {
		t.Fatal("emitter should be disabled without the env var")
	}

	// Emitting is a no-op and must not panic or create files.
	emitter.EmitChoicePrompt(choiceActions())
	emitter.EmitResult("ok", true, "")
}

// TestEmitFailClosed proves an append that cannot land prints the stderr
// sentinel and exits non-zero (plan §3.2). It re-execs itself as a child
// with the event-channel pointed at a directory, which is unwritable as a file.
func TestEmitFailClosed(t *testing.T) {
	if os.Getenv("TRUE_BDD_EMIT_FAILCLOSED_CHILD") == "1" {
		events.NewEmitter().EmitResult("ok", true, "")

		return
	}

	dir := filepath.Join(t.TempDir(), "eventsdir")

	err := os.Mkdir(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	//nolint:gosec // re-exec of the test binary with a fixed run filter
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestEmitFailClosed")

	cmd.Env = append(os.Environ(),
		"TRUE_BDD_EMIT_FAILCLOSED_CHILD=1",
		events.EventsFileEnv+"="+dir,
	)

	output, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if !asExitError(err, &exitErr) {
		t.Fatalf("child exit = %v, want non-zero exit; output=%s", err, output)
	}

	if !strings.Contains(string(output), events.FailClosedSentinel) {
		t.Fatalf("child output missing sentinel; output=%s", output)
	}
}

// asExitError reports whether err is a non-nil *exec.ExitError.
func asExitError(err error, target **exec.ExitError) bool {
	exitErr, ok := err.(*exec.ExitError) //nolint:errorlint // direct type check in test
	if ok {
		*target = exitErr
	}

	return ok
}
