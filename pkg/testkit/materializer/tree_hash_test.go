package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for rel, content := range files {
		path := filepath.Join(root, rel)

		err := os.MkdirAll(filepath.Dir(path), 0o755)
		if err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}

		err = os.WriteFile(path, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

func TestHashTreeSlashKeysAndSha256Values(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"a.txt":         "alpha",
		"nested/b.yaml": "beta",
		"nested/deep/c": "gamma",
	})

	hashes, err := HashTree(root)
	if err != nil {
		t.Fatalf("hash tree: %v", err)
	}

	if len(hashes) != 3 {
		t.Fatalf("expected 3 entries, got %d: %#v", len(hashes), hashes)
	}

	sum := sha256.Sum256([]byte("beta"))
	if hashes["nested/b.yaml"] != hex.EncodeToString(sum[:]) {
		t.Fatalf("nested/b.yaml hash = %q", hashes["nested/b.yaml"])
	}
}

func TestHashTreeExcludesRootTmpOnly(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tmp/runtime.log":     "noise",
		"tmp/deep/nested.log": "noise",
		"services/x/tmp/keep": "hashed",
		testProductPath:       "title: x",
	})

	hashes, err := HashTree(root)
	if err != nil {
		t.Fatalf("hash tree: %v", err)
	}

	for path := range hashes {
		if path == "tmp/runtime.log" || path == "tmp/deep/nested.log" {
			t.Fatalf("root tmp/** leaked into the hash: %s", path)
		}
	}

	if _, present := hashes["services/x/tmp/keep"]; !present {
		t.Fatal("nested tmp dirs must NOT be excluded (only root tmp/**)")
	}

	if _, present := hashes[testProductPath]; !present {
		t.Fatal("canonical file missing")
	}
}

func TestHashTreeEmptyDir(t *testing.T) {
	hashes, err := HashTree(t.TempDir())
	if err != nil {
		t.Fatalf("hash tree: %v", err)
	}

	if hashes == nil || len(hashes) != 0 {
		t.Fatalf("expected empty non-nil map, got %#v", hashes)
	}
}
