package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func TestLegacyCodexWriterMigrationRequiresTheExactHistoricalRequest(t *testing.T) {
	root := "../../../tests/bdd-cli/fixtures/build-code-fix-by-codex/cassettes/codex-002"

	manifest, err := readMeta(root)
	if err != nil {
		t.Fatal(err)
	}

	stdin, err := disk.Read(filepath.Join(root, "stdin"))
	if err != nil {
		t.Fatal(err)
	}

	args, prompt, hash := replayExpectation(manifest, string(stdin))
	if hash == manifest.RequestHash || !strings.Contains(prompt, "poll that process") ||
		!strings.Contains(strings.Join(args, " "), "--strict-config --ignore-rules --json") {
		t.Fatal("legacy writer request was not migrated")
	}

	if requestHash(args, []byte(prompt), cwdPlaceholder) != hash {
		t.Fatal("inconsistent migration")
	}

	for _, mutation := range []string{"changed plan", "changed command"} {
		_, _, got := replayExpectation(manifest, string(stdin)+mutation)
		if got != manifest.RequestHash {
			t.Fatal("modified historical request was migrated")
		}
	}

	manifest.Binary = "crush"

	_, _, got := replayExpectation(manifest, string(stdin))
	if got != manifest.RequestHash {
		t.Fatal("crush request was migrated")
	}
}
