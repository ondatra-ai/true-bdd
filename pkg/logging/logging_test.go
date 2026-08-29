package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
)

// jsonLines reads the file sink and fails unless every line parses. The
// appender trims slog's trailing newline before disk.Append adds its own; a
// regression there shows up as a blank line rather than a bad record.
func jsonLines(t *testing.T, path string) []map[string]any {
	t.Helper()

	raw, err := disk.Read(path)
	if err != nil {
		t.Fatalf("read sink: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	records := make([]map[string]any, 0, len(lines))

	for _, line := range lines {
		var record map[string]any

		err = json.Unmarshal([]byte(line), &record)
		if err != nil {
			t.Fatalf("sink line %q does not parse: %v", line, err)
		}

		records = append(records, record)
	}

	return records
}

func TestEveryRecordReachesTheFileAndOnlyInfoReachesTheStream(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "run.log.json")

	var stream bytes.Buffer

	log := slog.New(logging.Handler(&stream, path))
	log.Debug("quiet one", "n", 1)
	log.Info("loud one", "n", 2)
	log.Error("louder", "n", 3)

	records := jsonLines(t, path)
	if len(records) != 3 {
		t.Fatalf("file holds %d records, want 3", len(records))
	}

	text := stream.String()
	if strings.Contains(text, "quiet one") {
		t.Fatalf("Debug reached the stream: %q", text)
	}

	if !strings.Contains(text, "loud one") || !strings.Contains(text, "louder") {
		t.Fatalf("stream missing Info/Error: %q", text)
	}
}

// The BDD harness wraps Handler to tap "AI turn usage" on its way past, so a
// wrapper must see every record including the ones the stream drops.
type tap struct {
	slog.Handler

	seen *[]string
}

func (h tap) Handle(ctx context.Context, record slog.Record) error {
	*h.seen = append(*h.seen, record.Message)

	err := h.Handler.Handle(ctx, record)
	if err != nil {
		return fmt.Errorf("wrapped handler: %w", err)
	}

	return nil
}

func TestAWrappedHandlerSeesEveryRecord(t *testing.T) {
	t.Parallel()

	var (
		seen   []string
		stream bytes.Buffer
	)

	path := filepath.Join(t.TempDir(), "run.log.json")
	log := slog.New(tap{Handler: logging.Handler(&stream, path), seen: &seen})

	log.Debug("one")
	log.Info("two")

	if len(seen) != 2 || seen[0] != "one" || seen[1] != "two" {
		t.Fatalf("tap saw %v, want [one two]", seen)
	}
}

// A script has no reader folding its run back into turns, so it installs a
// text-only sink rather than growing a write-only file on every invocation.
func TestAnEmptyPathIsTextOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var stream bytes.Buffer

	log := slog.New(logging.Handler(&stream, ""))
	log.Error("no sink here", "n", 1)

	if !strings.Contains(stream.String(), "no sink here") {
		t.Fatalf("stream = %q", stream.String())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("text-only handler wrote %d files", len(entries))
	}
}

// Install is what the seven scripts/ programs call, and its two attributes are
// what makes their one shared Task log parseable. Not parallel: it replaces
// the process default.
func TestInstallStampsTheWriterAndItsRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task.log.json")

	logging.Install(logging.Stderr, path, "gates")
	slog.Info("Gate", "name", "alint")

	records := jsonLines(t, path)
	if len(records) != 1 {
		t.Fatalf("file holds %d records, want 1", len(records))
	}

	if got := records[0]["tool"]; got != "gates" {
		t.Errorf("tool = %v, want gates", got)
	}

	if got := records[0]["run"]; got != logging.Run() {
		t.Errorf("run = %v, want %q", got, logging.Run())
	}
}

// The engine passes no tool, binds the Stdout text handler, and 22 scenario
// steps regex those lines against goldens: a run id there is a golden diff
// every run.
func TestInstallStampsNothingWithoutATool(t *testing.T) {
	path := filepath.Join(t.TempDir(), "engine.log.json")

	logging.Install(logging.Stdout, path, "")
	slog.Info("engine says")

	records := jsonLines(t, path)
	if _, ok := records[0]["run"]; ok {
		t.Errorf("record carries a run id: %v", records[0])
	}

	if _, ok := records[0]["tool"]; ok {
		t.Errorf("record carries a tool: %v", records[0])
	}
}

// E2E-019 asserts the engine dispatched no AI turns by reading its log, so a
// run that records nothing must still leave a file to read.
func TestInstallCreatesTheFileUpFront(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unwritten.log.json")

	logging.Install(logging.Stderr, path, "history")

	_, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Install left no file to read (%v)", err)
	}
}
