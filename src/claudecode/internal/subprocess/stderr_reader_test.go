package subprocess_test

import (
	"os"
	"strings"
	"testing"

	"bdd-cli/src/claudecode/internal/subprocess"
)

func TestReadStderr_NilStderr(t *testing.T) {
	transport := subprocess.NewTestTransportWithChannels()
	defer subprocess.CloseTestTransport(transport)

	result := subprocess.ReadStderrForTest(transport)
	if result != "" {
		t.Errorf("expected empty string for nil stderr, got %q", result)
	}
}

func TestReadStderr_ReturnsContent(t *testing.T) {
	transport := subprocess.NewTestTransportWithChannels()
	defer subprocess.CloseTestTransport(transport)

	// Create a temp file with some error content
	stderrFile, err := os.CreateTemp(t.TempDir(), "test_stderr_*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	expected := "Error: authentication failed\nPlease run 'claude login'"

	_, writeErr := stderrFile.WriteString(expected)
	if writeErr != nil {
		t.Fatalf("failed to write to temp file: %v", writeErr)
	}

	subprocess.SetTestTransportStderr(transport, stderrFile)

	result := subprocess.ReadStderrForTest(transport)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestReadStderr_TruncatesLargeContent(t *testing.T) {
	transport := subprocess.NewTestTransportWithChannels()
	defer subprocess.CloseTestTransport(transport)

	stderrFile, err := os.CreateTemp(t.TempDir(), "test_stderr_large_*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Write 15KB of content (exceeds 10KB limit)
	largeContent := strings.Repeat("x", 15*1024)

	_, writeErr := stderrFile.WriteString(largeContent)
	if writeErr != nil {
		t.Fatalf("failed to write to temp file: %v", writeErr)
	}

	subprocess.SetTestTransportStderr(transport, stderrFile)

	result := subprocess.ReadStderrForTest(transport)
	if len(result) > 10*1024 {
		t.Errorf("expected result to be at most 10KB, got %d bytes", len(result))
	}
}

func TestReadStderr_ReadsFromStartAfterWrite(t *testing.T) {
	transport := subprocess.NewTestTransportWithChannels()
	defer subprocess.CloseTestTransport(transport)

	stderrFile, err := os.CreateTemp(t.TempDir(), "test_stderr_seek_*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	expected := "some error output"

	_, writeErr := stderrFile.WriteString(expected)
	if writeErr != nil {
		t.Fatalf("failed to write to temp file: %v", writeErr)
	}

	subprocess.SetTestTransportStderr(transport, stderrFile)

	// Call twice to verify it seeks back to start each time
	result1 := subprocess.ReadStderrForTest(transport)
	result2 := subprocess.ReadStderrForTest(transport)

	if result1 != expected {
		t.Errorf("first read: expected %q, got %q", expected, result1)
	}

	if result2 != expected {
		t.Errorf("second read: expected %q, got %q", expected, result2)
	}
}
