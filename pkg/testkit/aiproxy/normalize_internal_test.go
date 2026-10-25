package main

import (
	"os"
	"strings"
	"testing"
)

const claudeRestricted = "--restricted"
const claudeSessionID = "--session-id"
const claudeAddDir = "--add-dir"

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
		"tmp/run/codex-turn002-codex-home/auth.json",
		"tmp/codex-go-build/cache.dat",
		"tmp/codex-shell-home/state",
		"tmp/codex-tmp/go-build001/output",
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

func TestClaudeWriterSessionDoesNotStaleReplay(t *testing.T) {
	t.Parallel()

	first := []string{claudeRestricted, claudeSessionID, "11111111-1111-4111-8111-111111111111", claudeAddDir,
		"/home/.claude/projects/-first/11111111-1111-4111-8111-111111111111/tool-results", "-p", "repair"}

	second := []string{claudeRestricted, claudeSessionID, "22222222-2222-4222-8222-222222222222", claudeAddDir,
		"/home/.claude/projects/-second/22222222-2222-4222-8222-222222222222/tool-results", "-p", "repair"}
	if requestHash(first, nil, "/first") != requestHash(second, nil, "/second") {
		t.Fatal("session-specific spill directory changed writer request identity")
	}

	second[len(second)-1] = "different plan"
	if requestHash(first, nil, "/first") == requestHash(second, nil, "/second") {
		t.Fatal("changed prompt was ignored")
	}
}

func TestClaudeWriterToolchainDoesNotStaleReplay(t *testing.T) {
	first := []string{claudeRestricted, claudeSessionID, "first", claudeAddDir, "/opt/homebrew/Cellar/go/1.27.1/libexec",
		claudeAddDir, "/Users/alice/go/pkg/mod", claudeAddDir, "/Users/alice/.claude/projects/host/first/tool-results",
		"-p", "repair /Users/alice/project/services"}
	second := []string{claudeRestricted, claudeSessionID, "second", claudeAddDir, "/usr/local/go",
		claudeAddDir, "/var/cache/go-mod", claudeAddDir, "/home/bob/.claude/projects/host/second/tool-results",
		"-p", "repair /home/bob/project/services"}

	t.Setenv("HOME", "/Users/alice")

	recorded := requestHash(first, nil, "/host")

	t.Setenv("HOME", "/home/bob")

	if replayed := requestHash(second, nil, "/host"); recorded != replayed {
		t.Fatalf("writer request went stale after relocating Go and home: %s != %s", recorded, replayed)
	}

	second[len(second)-1] = "repair a different service"
	if requestHash(second, nil, "/host") == recorded {
		t.Fatal("changed repair request was ignored")
	}
}
