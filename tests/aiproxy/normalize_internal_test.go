package main

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizeTextReplacesCwdAndRunDir(t *testing.T) {
	t.Parallel()

	in := "read /work/fixture/docs/story.yaml and write /work/fixture/tmp/2026-08-14-12-00-01-4242/result.md"
	got := normalizeText(in, "/work/fixture")

	want := "read {{CWD}}/docs/story.yaml and write {{CWD}}/tmp/{{RUN_DIR}}/result.md"
	if got != want {
		t.Fatalf("normalizeText:\n got %q\nwant %q", got, want)
	}
}

func TestRequestHashStableAcrossRunLocations(t *testing.T) {
	t.Parallel()

	argvA := []string{"run", "--cwd", "/runs/a"}
	argvB := []string{"run", "--cwd", "/runs/b"}
	stdinA := []byte("story at /runs/a/docs/s.yaml, artifacts in tmp/2026-01-01-00-00-00-1")
	stdinB := []byte("story at /runs/b/docs/s.yaml, artifacts in tmp/2026-02-02-00-00-00-2")

	hashA := requestHash(argvA, stdinA, "/runs/a")
	hashB := requestHash(argvB, stdinB, "/runs/b")

	if hashA != hashB {
		t.Fatalf("hashes differ across run locations: %s vs %s", hashA, hashB)
	}

	changed := requestHash(argvA, []byte("a different prompt"), "/runs/a")
	if changed == hashA {
		t.Fatal("hash did not change with the prompt")
	}

	if !strings.HasPrefix(hashA, "sha256:") {
		t.Fatalf("hash missing prefix: %s", hashA)
	}
}

func TestFSDiffRoundTripAcrossRuns(t *testing.T) {
	t.Parallel()

	recordCwd := "/runs/record/us-refine-fix-steps"
	recorded := "tmp/2026-08-14-21-46-41-78898/01-result.yaml"

	normalized := normalizeText(recorded, recordCwd)
	if normalized != "tmp/{{RUN_DIR}}/01-result.yaml" {
		t.Fatalf("normalize path: %q", normalized)
	}

	live, err := denormalize(normalized, "/runs/replay/us-refine-fix-steps", "2026-08-14-21-49-58-82450")
	if err != nil {
		t.Fatalf("denormalize: %v", err)
	}

	if live != "tmp/2026-08-14-21-49-58-82450/01-result.yaml" {
		t.Fatalf("denormalized path: %q", live)
	}

	_, err = denormalize(normalized, "/runs/replay", "")
	if err == nil {
		t.Fatal("denormalize without a run dir should fail loudly")
	}
}

func TestIsEngineOwned(t *testing.T) {
	t.Parallel()

	owned := []string{
		"tmp/true-bdd.log.json",
		"tmp/aiproxy-state/cursor-claude",
		"tmp/2026-08-14-21-46-41-78898/crush-config-abc/crush-data/db.sqlite",
	}
	for _, path := range owned {
		if !isEngineOwned(path) {
			t.Errorf("expected engine-owned: %s", path)
		}
	}

	recorded := []string{
		"tmp/2026-08-14-21-46-41-78898/01-result.yaml",
		"docs/product/stories/96.5-document-summary-steps.yaml",
	}
	for _, path := range recorded {
		if isEngineOwned(path) {
			t.Errorf("must be recorded, not filtered: %s", path)
		}
	}
}

func TestNextCallIndexIncrementsPerBinary(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()

	for want := 1; want <= 3; want++ {
		got, err := nextCallIndex(stateDir, "claude")
		if err != nil {
			t.Fatalf("nextCallIndex: %v", err)
		}

		if got != want {
			t.Fatalf("claude call %d: got index %d", want, got)
		}
	}

	got, err := nextCallIndex(stateDir, "crush")
	if err != nil {
		t.Fatalf("nextCallIndex crush: %v", err)
	}

	if got != 1 {
		t.Fatalf("crush cursor not independent: got %d", got)
	}
}

// Cassettes are COMMITTED, so a recording that carries the recorder's
// home directory publishes their username. These pin the placeholder and
// its one dangerous edge: a sibling path that merely shares the prefix.
func TestNormalizeReplacesHome(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory on this machine")
	}

	got := normalizeText(home+"/.claude/plugins/cache", "/somewhere/else")
	if strings.Contains(got, home) {
		t.Fatalf("home survived normalization: %q", got)
	}

	if want := homePlaceholder + "/.claude/plugins/cache"; got != want {
		t.Fatalf("normalizeText = %q, want %q", got, want)
	}
}

func TestNormalizeLeavesSiblingPathsAlone(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory on this machine")
	}

	// /Users/peter must not rewrite the unrelated /Users/peterson.
	sibling := home + "son/project"

	if got := normalizeText(sibling, "/somewhere/else"); got != sibling {
		t.Fatalf("sibling path was mangled: %q became %q", sibling, got)
	}
}

// Recording normalizes against ITS home and replay denormalizes against
// ITS OWN, which is what lets a cassette recorded under /Users/alice
// replay under /home/bob.
func TestHomeRoundTrip(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory on this machine")
	}

	original := home + "/.codex/sessions/x.jsonl"

	back, err := denormalize(normalizeText(original, "/cwd"), "/cwd", "run-dir")
	if err != nil {
		t.Fatalf("denormalize: %v", err)
	}

	if back != original {
		t.Fatalf("round trip changed the path: %q -> %q", original, back)
	}
}
