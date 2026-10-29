package main

import (
	"strings"
	"testing"
)

func TestRecordingDropsPrivateGoTelemetryCounterNames(t *testing.T) {
	t.Parallel()

	data := []byte(`{"type":"item.completed","item":{"aggregated_output":"` +
		`tmp/codex-shell-home/Library/Application Support/go/telemetry/local/` +
		`go@go1.27.1-darwin-arm64-2026-10-29.v1.count\\nkeep expected 6, got 3"}}`)

	sanitized := string(sanitizeStream(data))
	if strings.Contains(sanitized, "go@go1.27.1") {
		t.Fatal("recording retained transient telemetry counter name")
	}

	if !strings.Contains(sanitized, "keep expected 6, got 3") {
		t.Fatal("failure evidence changed")
	}
}
