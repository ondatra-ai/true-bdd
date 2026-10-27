package release_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/scripts/release"
)

func TestBuildExportsCommittedSourceAndProducesFourChecksummedArchives(t *testing.T) {
	root := sourceRepository(t)
	write(t, filepath.Join(root, "untracked"), "must not ship")
	write(t, filepath.Join(root, "go.mod"), "dirty tree must not build")

	builder := filepath.Join(os.Getenv("PATH"), "go")

	err := os.Remove(builder)
	if err != nil {
		t.Fatal(err)
	}

	write(t, builder, `#!/bin/sh
set -eu
identity=github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/buildinfo
test "$1" = build
test "$2" = -trimpath
case "$3" in "-ldflags=-X $identity.version=test -X $identity.commit="????????????????????????????????????????) ;;
 *) exit 9 ;; esac
test "$4" = -o
test "$GOWORK" = off
test ! -e untracked
/usr/bin/grep -q 'module github.com/ondatra-ai/true-bdd/services/true-bdd$' go.mod
printf '#!/bin/sh\necho %s/%s\n' "$GOOS" "$GOARCH" > "$5"
`)

	err = os.Chmod(builder, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	artifacts := filepath.Join(t.TempDir(), "artifacts")

	err = release.Build(context.Background(), root, "test", artifacts)
	if err != nil {
		t.Fatal(err)
	}

	checksums, err := os.ReadFile(filepath.Join(artifacts, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if len(strings.Fields(string(checksums))) != 8 {
		t.Fatalf("need exactly four checksums: %s", checksums)
	}

	metadata, err := os.ReadFile(filepath.Join(artifacts, "release.json"))
	if err != nil {
		t.Fatal(err)
	}

	var decoded release.Metadata

	err = json.Unmarshal(metadata, &decoded)
	if err != nil {
		t.Fatal(err)
	}

	if len(decoded.Commit) != 40 {
		t.Fatalf("source must be immutable: %s", metadata)
	}

	_, err = os.Stat(filepath.Join(artifacts, "true-bdd-resources_test.tar.gz"))
	if !os.IsNotExist(err) {
		t.Fatal("resources archive must not be produced")
	}
}

func TestBuildTagRefusesATagOfAnotherFormOrCommit(t *testing.T) {
	root := sourceRepository(t)
	out := filepath.Join(t.TempDir(), "artifacts")

	err := release.BuildTag(context.Background(), root, "v0.1.0", out)
	if err == nil || !strings.Contains(err.Error(), "services/true-bdd/vX.Y.Z") {
		t.Fatalf("a bare version tag was accepted: %v", err)
	}

	for _, args := range [][]string{
		{"tag", "services/true-bdd/v0.1.0"},
		{"-c", "user.name=Test", "-c", "user.email=test@localhost", "-c", "commit.gpgsign=false",
			"commit", "--allow-empty", "-m", "after the tag"},
	} {
		command := exec.CommandContext(t.Context(), "git", args...)

		command.Dir = root
		command.Env = git.ScrubEnvironment(os.Environ())

		output, gitErr := command.CombinedOutput()
		if gitErr != nil {
			t.Fatalf("git %v: %s: %v", args, output, gitErr)
		}
	}

	err = release.BuildTag(context.Background(), root, "services/true-bdd/v0.1.0", out)
	if err == nil || !strings.Contains(err.Error(), "not HEAD") {
		t.Fatalf("a tag naming another commit was accepted: %v", err)
	}
}
