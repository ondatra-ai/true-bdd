package state_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/state"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

func TestLogWritesTheEngineShapeOnTheDerivedPath(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("CLAUDE_HISTORY_ROLE", "claude")

	opened, err := state.Task(repo, session, "a prompt")
	if err != nil {
		t.Fatalf("Task: %v", err)
	}

	state.Log(repo).Info("gate finished", "gate", "alint", "duration_ms", 25)

	raw, err := os.ReadFile(state.LogFile(repo, opened))
	if err != nil {
		t.Fatalf("reading the log: %v", err)
	}

	var record struct {
		Time       string `json:"time"`
		Level      string `json:"level"`
		Msg        string `json:"msg"`
		DurationMs int64  `json:"duration_ms"`
	}

	err = json.Unmarshal(raw, &record)
	if err != nil {
		t.Fatalf("the log is not JSON: %v", err)
	}

	if record.Msg != "gate finished" || record.Level != "INFO" || record.DurationMs != 25 || record.Time == "" {
		t.Fatalf("record = %+v, want the engine's time/level/msg/duration_ms shape", record)
	}
}

func TestLogDiscardsWithNoTask(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("CLAUDE_HISTORY_ROLE", "claude")

	state.Log(repo).Info("nowhere to attribute this")

	entries, err := os.ReadDir(state.HistoryDir(repo))
	if err == nil && len(entries) > 0 {
		t.Fatalf("%v was written with no Task open", entries)
	}
}

func TestLogDiscardsWhenCaptureIsOff(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("CLAUDE_HISTORY_ROLE", "0")

	opened, err := state.Task(repo, session, "a prompt")
	if err != nil {
		t.Fatalf("Task: %v", err)
	}

	state.Log(repo).Info("capture is off")

	if fileExists(state.LogFile(repo, opened)) {
		t.Fatal("the log was written with CLAUDE_HISTORY_ROLE=0")
	}
}
