package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	storyPath = "docs/product/stories/96.1-story.yaml"
	// The two CLIs a fixture can spawn, named once.
	claudeCLI = "claude"
	crushCLI  = "crush"
)

func changed(path, kind, body string) FileChange {
	return FileChange{Path: path, Kind: kind, After: []byte(body)}
}

func TestCompareGoldenAcceptsAnIdenticalRun(t *testing.T) {
	t.Parallel()

	diff := []FileChange{
		changed(storyPath, "modified", "id: 96.1\n"),
		changed("tmp/2026-01-01-00-00-00-1/prompt.txt", "created", "whatever"),
	}

	golden := NewGoldenTree("fx", diff)

	// The scratch entry must not even be recorded: its path carries a
	// per-run timestamp, so pinning it would fail every future run.
	if len(golden.Files) != 1 {
		t.Fatalf("recorded %d file(s), want 1 (tmp/ excluded)", len(golden.Files))
	}

	failures := CompareGolden(&golden, diff)
	if len(failures) != 0 {
		t.Fatalf("identical run reported %d failure(s): %v", len(failures), failures)
	}
}

// The scratch directory is named after the moment the run started, so
// the same run replayed tomorrow writes different paths. Excluding it is
// what makes the comparison stable rather than a clock check.
func TestCompareGoldenIgnoresScratchPathsThatMoved(t *testing.T) {
	t.Parallel()

	recorded := []FileChange{
		changed(storyPath, "modified", "id: 96.1\n"),
		changed("tmp/2026-01-01-00-00-00-1/prompt.txt", "created", "one"),
	}
	replayed := []FileChange{
		changed(storyPath, "modified", "id: 96.1\n"),
		changed("tmp/2026-06-30-23-59-59-2/prompt.txt", "created", "another"),
	}

	golden := NewGoldenTree("fx", recorded)

	failures := CompareGolden(&golden, replayed)
	if len(failures) != 0 {
		t.Fatalf("scratch drift reported %d failure(s): %v", len(failures), failures)
	}
}

func TestCompareGoldenDetects(t *testing.T) {
	t.Parallel()

	recorded := []FileChange{changed(storyPath, "modified", "id: 96.1\nname: first\n")}
	golden := NewGoldenTree("fx", recorded)

	cases := []struct {
		name    string
		actual  []FileChange
		wantSub string
	}{
		{
			name:    "content that moved",
			actual:  []FileChange{changed(storyPath, "modified", "id: 96.1\nname: second\n")},
			wantSub: "differs from the recording",
		},
		{
			name:    "a file the run stopped producing",
			actual:  nil,
			wantSub: "was not produced by this run",
		},
		{
			// A regression that only ADDS output is still a regression.
			name: "a file the run started producing",
			actual: []FileChange{
				changed(storyPath, "modified", "id: 96.1\nname: first\n"),
				changed("docs/scenarios.yaml", "created", "scenarios: []\n"),
			},
			wantSub: "is not in the recording",
		},
		{
			name:    "a change of kind",
			actual:  []FileChange{changed(storyPath, "created", "id: 96.1\nname: first\n")},
			wantSub: "was modified in the recording, created in this run",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			failures := CompareGolden(&golden, testCase.actual)
			if len(failures) != 1 {
				t.Fatalf("got %d failure(s), want 1: %v", len(failures), failures)
			}

			if !strings.Contains(failures[0], testCase.wantSub) {
				t.Fatalf("failure %q does not mention %q", failures[0], testCase.wantSub)
			}
		})
	}
}

// A differing file has to say WHERE it differs; two digests are not a
// diagnosis.
func TestCompareGoldenShowsTheFirstDifference(t *testing.T) {
	t.Parallel()

	golden := NewGoldenTree("fx", []FileChange{changed(storyPath, "modified", "a\nb\nc\n")})

	failures := CompareGolden(&golden, []FileChange{changed(storyPath, "modified", "a\nB\nc\n")})
	if len(failures) != 1 {
		t.Fatalf("got %d failure(s), want 1", len(failures))
	}

	if !strings.Contains(failures[0], "first difference at line 2") {
		t.Fatalf("failure does not locate the difference:\n%s", failures[0])
	}

	if !strings.Contains(failures[0], "- b") || !strings.Contains(failures[0], "+ B") {
		t.Fatalf("failure does not show both sides of the difference:\n%s", failures[0])
	}

	// The agreeing line after the divergence appears once, unmarked —
	// a ± pair for identical lines is what made this unreadable.
	if strings.Contains(failures[0], "- c") {
		t.Fatalf("identical lines were rendered as differences:\n%s", failures[0])
	}
}

func TestCheckCassettesConsumed(t *testing.T) {
	t.Parallel()

	cassettes := t.TempDir()
	for _, name := range []string{claudeCLI + "-001", claudeCLI + "-002", crushCLI + "-001"} {
		err := os.MkdirAll(filepath.Join(cassettes, name), 0o755)
		if err != nil {
			t.Fatalf("seed cassette: %v", err)
		}
	}

	// golden.json sits in the same directory and is not a call.
	err := os.WriteFile(filepath.Join(cassettes, GoldenFile), []byte("{}"), 0o644)
	if err != nil {
		t.Fatalf("seed golden: %v", err)
	}

	cases := []struct {
		name         string
		cursors      map[string]string
		wantFailures int
		wantSub      string
	}{
		{
			name:    "every recorded call was served",
			cursors: map[string]string{claudeCLI: "2", crushCLI: "1"},
		},
		{
			// The divergence a request hash cannot see: the engine stops
			// early, every call it DID make matches, and the run ends green.
			name:         "the engine stopped a turn early",
			cursors:      map[string]string{claudeCLI: "1", crushCLI: "1"},
			wantFailures: 1,
			wantSub:      "made 1 claude call(s) but 2 were recorded",
		},
		{
			name:         "a binary was never spawned",
			cursors:      map[string]string{claudeCLI: "2"},
			wantFailures: 1,
			wantSub:      "made 0 crush call(s) but 1 were recorded",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			state := t.TempDir()
			for binary, value := range testCase.cursors {
				writeErr := os.WriteFile(filepath.Join(state, "cursor-"+binary), []byte(value), 0o644)
				if writeErr != nil {
					t.Fatalf("seed cursor: %v", writeErr)
				}
			}

			failures := CheckCassettesConsumed(cassettes, state)
			if len(failures) != testCase.wantFailures {
				t.Fatalf("got %d failure(s), want %d: %v", len(failures), testCase.wantFailures, failures)
			}

			if testCase.wantSub != "" && !strings.Contains(failures[0], testCase.wantSub) {
				t.Fatalf("failure %q does not mention %q", failures[0], testCase.wantSub)
			}
		})
	}
}
