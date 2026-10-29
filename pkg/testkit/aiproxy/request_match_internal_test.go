package main

import (
	"errors"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"path/filepath"
	"sync"
	"testing"
)

const requestAlpha = "request-alpha"
const requestBeta = "request-beta"

func TestReplayClaimsByRequestAndConsumesIdenticalOccurrences(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	cfg := config{Cassettes: filepath.Join(root, "cassettes"), StateDir: filepath.Join(root, "state")}
	for index, request := range []string{requestAlpha, requestBeta, requestAlpha} {
		manifest := &meta{Binary: "claude", RequestHash: requestHash([]string{request}, nil, root)}

		err := writeCassette(filepath.Join(cfg.Cassettes, cassetteName("claude", index+1)), manifest, nil, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	for index, request := range []string{requestBeta, requestAlpha, requestAlpha} {
		dir, _, err := claimRequest(cfg, "claude", []string{request}, nil, root)
		if err != nil {
			t.Fatal(err)
		}

		want := []string{"claude-002", "claude-001", "claude-003"}[index]
		if filepath.Base(dir) != want {
			t.Fatalf("got %s, want %s", dir, want)
		}
	}

	_, _, err := claimRequest(cfg, "claude", []string{requestAlpha}, nil, root)
	if !errors.Is(err, errCassetteStale) {
		t.Fatalf("exhausted request: %v", err)
	}
}

func TestReplayConcurrentIdenticalRequestsClaimEachCassetteOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := config{Cassettes: filepath.Join(root, "cassettes"), StateDir: filepath.Join(root, "state")}

	const count = 8
	for index := range count {
		manifest := &meta{Binary: "claude", RequestHash: requestHash([]string{"same"}, nil, root)}

		err := writeCassette(filepath.Join(cfg.Cassettes, cassetteName("claude", index+1)), manifest, nil, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	results := make(chan string, count)

	var group sync.WaitGroup
	for range count {
		group.Go(func() {
			dir, _, err := claimRequest(cfg, "claude", []string{"same"}, nil, root)
			if err != nil {
				t.Error(err)
			}

			results <- dir
		})
	}

	group.Wait()
	close(results)

	unique := map[string]bool{}
	for dir := range results {
		if unique[dir] {
			t.Fatalf("cassette claimed twice: %s", dir)
		}

		unique[dir] = true
	}

	if len(unique) != count {
		t.Fatalf("claimed %d, want %d", len(unique), count)
	}
}

func TestValidationRecordingKeepsOnlyOwnCodexAnswerAmongEngineArtifacts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_HISTORY_ROLE", "")

	run := filepath.Join("tmp", "2026-10-29-12-00-00-123")

	answer := filepath.Join(run, "own-codex-answer.txt")
	for _, rel := range []string{answer, filepath.Join(run, "other-codex-answer.txt"),
		filepath.Join(run, ".true-bdd.tmp.artifact"), "docs/product.yaml", "tmp/aiproxy-state/consumed-claude"} {
		err := disk.Write(filepath.Join(root, rel), []byte("content"), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	snapshot, err := recordingSnapshot(root, []string{"-s", "read-only", "-o", filepath.Join(root, answer)})
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot) != 2 || string(snapshot[answer]) != "content" || string(snapshot["docs/product.yaml"]) != "content" {
		t.Fatalf("snapshot must keep own answer and documents only: %v", snapshot)
	}
}
