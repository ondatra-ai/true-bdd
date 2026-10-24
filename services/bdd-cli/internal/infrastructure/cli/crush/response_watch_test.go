package crush_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/cli/crush"
)

func TestResponseWatchStopsAtTenMinutesAndWarnsOnce(t *testing.T) {
	start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "logs", "crush.log")

	var logs bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	watch := crush.ResponseWatch{LogPath: path, Label: "crush-turn002"}
	if watch.Check(start) {
		t.Fatal("absent log was timed")
	}

	appendRecord(t, path, start, "ModelProvider called")

	if watch.Check(start.Add(10*time.Minute - time.Nanosecond)) {
		t.Fatal("stopped before ten minutes")
	}

	if !watch.Check(start.Add(10 * time.Minute)) {
		t.Fatal("open response did not stop")
	}

	watch.Check(start.Add(time.Hour))

	for _, want := range []string{`level=WARN msg="Writer response stalled"`, `label=crush-turn002`, `minutes=10`,
		"log_path=" + path} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log %q missing %q", logs.String(), want)
		}
	}

	if strings.Count(logs.String(), "Writer response stalled") != 1 {
		t.Fatal("stall warned more than once")
	}
}

func appendRecord(t *testing.T, path string, at time.Time, message string) {
	t.Helper()

	record := fmt.Sprintf("{\"time\":%q,\"msg\":%q}", at.Format(time.RFC3339Nano), message)

	err := disk.Append(path, []byte(record), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}

func TestResponseWatchLeavesToolsAndOtherRecordsUnbounded(t *testing.T) {
	for _, messages := range [][]string{
		{"Starting up"},
		{"ModelProvider called", "Hook completed"},
		{"ModelProvider called", "Response finished"},
	} {
		t.Run(strings.Join(messages, "/"), func(t *testing.T) {
			start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
			path := filepath.Join(t.TempDir(), "crush.log")

			watch := crush.ResponseWatch{LogPath: path}
			for _, message := range messages {
				appendRecord(t, path, start, message)

				if watch.Check(start) {
					t.Fatal("fresh response stopped")
				}
			}

			if watch.Check(start.Add(1000 * time.Hour)) {
				t.Fatal("non-model activity was timed")
			}
		})
	}
}

func TestResponseWatchAllowsResponsesThatKeepEnding(t *testing.T) {
	start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "crush.log")

	watch := crush.ResponseWatch{LogPath: path}
	for range 20 {
		appendRecord(t, path, start, "ModelProvider called")

		if watch.Check(start.Add(9 * time.Minute)) {
			t.Fatal("short response stopped")
		}

		appendRecord(t, path, start.Add(9*time.Minute), "Hook completed")

		if watch.Check(start.Add(24 * time.Hour)) {
			t.Fatal("long tool execution stopped")
		}

		start = start.Add(24 * time.Hour)
	}
}

func TestResponseWatchConsumesOnlyCompleteRecordsAndResetsAfterTruncation(t *testing.T) {
	start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "crush.log")
	watch := crush.ResponseWatch{LogPath: path}
	appendRecord(t, path, start, "ModelProvider called")

	if watch.Check(start) {
		t.Fatal("fresh response stopped")
	}

	err := disk.AppendBytes(path, []byte(`{"msg":"Hook completed"}`), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	if watch.Check(start.Add(9 * time.Minute)) {
		t.Fatal("fresh response stopped")
	}

	err = disk.AppendBytes(path, []byte("\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	if watch.Check(start.Add(time.Hour)) {
		t.Fatal("complete hook did not close response")
	}

	err = disk.Write(path, nil, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	if watch.Check(start.Add(2 * time.Hour)) {
		t.Fatal("truncated log timed")
	}

	appendRecord(t, path, start.Add(2*time.Hour), "ModelProvider called")

	if !watch.Check(start.Add(2*time.Hour + 10*time.Minute)) {
		t.Fatal("new response not watched")
	}
}
