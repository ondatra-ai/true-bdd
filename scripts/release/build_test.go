package release_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
test "$1" = build
test "$2" = -trimpath
test "$3" = -o
test ! -e untracked
/usr/bin/grep -q 'module github.com/ondatra-ai/true-bdd' go.mod
printf '#!/bin/sh\necho %s/%s\n' "$GOOS" "$GOARCH" > "$4"
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
