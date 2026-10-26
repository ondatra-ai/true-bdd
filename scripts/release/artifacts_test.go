package release_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/release"
)

func TestWrongChecksumIsRefused(t *testing.T) {
	t.Parallel()
	artifacts := artifactSet(t, "#!/bin/sh\nexit 0\n")
	write(t, filepath.Join(artifacts, "true-bdd_test_linux_arm64.tar.gz"), "tampered")

	_, err := release.Rehearse(context.Background(), t.TempDir(), artifacts, release.Options{})
	if err == nil || !strings.Contains(err.Error(), "wrong checksum") {
		t.Fatalf("tampered archive accepted: %v", err)
	}
}

func TestBinaryThatDoesNotRunIsRefused(t *testing.T) {
	t.Parallel()
	artifacts := artifactSet(t, "#!/bin/sh\necho broken >&2\nexit 17\n")

	_, err := release.Rehearse(context.Background(), t.TempDir(), artifacts, release.Options{})
	if err == nil || !strings.Contains(err.Error(), "binary") {
		t.Fatalf("broken binary accepted: %v", err)
	}
}

func artifactSet(t *testing.T, binary string) string {
	t.Helper()

	var buffer bytes.Buffer

	compressed := gzip.NewWriter(&buffer)

	archive := tar.NewWriter(compressed)

	err := archive.WriteHeader(&tar.Header{Name: "true-bdd", Mode: 0o755, Size: int64(len(binary))})
	if err != nil {
		t.Fatal(err)
	}

	_, err = archive.Write([]byte(binary))
	if err != nil {
		t.Fatal(err)
	}

	err = archive.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = compressed.Close()
	if err != nil {
		t.Fatal(err)
	}

	artifacts := t.TempDir()

	var checksums strings.Builder

	for _, platform := range []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"} {
		name := "true-bdd_test_" + platform + ".tar.gz"

		err := os.WriteFile(filepath.Join(artifacts, name), buffer.Bytes(), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		fmt.Fprintf(&checksums, "%x  %s\n", sha256.Sum256(buffer.Bytes()), name)
	}

	write(t, filepath.Join(artifacts, "release.json"), `{"version":"test","commit":"HEAD"}`)
	write(t, filepath.Join(artifacts, "checksums.txt"), checksums.String())

	return artifacts
}
