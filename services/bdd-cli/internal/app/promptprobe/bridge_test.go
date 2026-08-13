package promptprobe_test

// Test bridge (plan §4): drives the real collector through a temp event-channel
// file (the SAME emitter the remote tails), then reads back the emitted prompt
// kinds + ids so the test can assert the choice → clarify → freetext contract.

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/promptprobe"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/events"
)

// runProbe drives one prompt-probe execution: it routes the probe's prompt
// emissions through a temp event-channel file (the same emitter the remote
// tails), then reads back the emitted prompt kinds + ids alongside the answers
// the collector recorded from stdin.
func runProbe(t *testing.T, stdin io.Reader) ProbeRun {
	t.Helper()

	eventsPath := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(events.EventsFileEnv, eventsPath)

	recorded := promptprobe.Drive(stdin)
	kinds, ids := readEmittedPrompts(t, eventsPath)

	return ProbeRun{PromptKinds: kinds, PromptIDs: ids, Recorded: recorded}
}

func readEmittedPrompts(t *testing.T, path string) ([]string, []string) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open events file: %v", err)
	}

	defer func() { _ = file.Close() }()

	var kinds, ids []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event struct {
			Type     string `json:"type"`
			Kind     string `json:"kind"`
			PromptID string `json:"prompt_id"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}

		if event.Type == "prompt" {
			kinds = append(kinds, event.Kind)
			ids = append(ids, event.PromptID)
		}
	}

	err = scanner.Err()
	if err != nil {
		t.Fatalf("scan events file: %v", err)
	}

	return kinds, ids
}
