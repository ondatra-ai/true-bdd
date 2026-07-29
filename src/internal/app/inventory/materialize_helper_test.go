package inventory_test

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"
)

// fixtureManifest mirrors the subset of a Playwright fixture.yaml the
// golden tests need to reproduce base:engine overlay + remove + input,
// so the scanner is exercised against the EXACT trees P3/P4 render-assert
// (plan §4.3/§4.6).
type fixtureManifest struct {
	Base   string   `yaml:"base"`
	Input  string   `yaml:"input"`
	Remove []string `yaml:"remove"`
}

// repoRoot walks up from this test file to the module root (the directory
// holding go.mod).
func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	dir := filepath.Dir(file)
	for {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from test file")
		}

		dir = parent
	}
}

// materialize reproduces a harness fixture into a fresh tmpdir: it copies
// the engine seed (true-bdd/) for base:engine, applies the manifest's
// remove list, then overlays the fixture input tree. It returns the
// canonical host folder the scanner runs against.
func materialize(t *testing.T, fixture string) string {
	t.Helper()

	root := repoRoot(t)
	fixtureDir := filepath.Join(root, "harness", "tests", "e2e", "fixtures", fixture)
	manifest := readManifest(t, filepath.Join(fixtureDir, "fixture.yaml"))
	target := t.TempDir()

	if manifest.Base == "engine" {
		copyTree(t, filepath.Join(root, "true-bdd"), filepath.Join(target, "true-bdd"))
	}

	for _, rel := range manifest.Remove {
		err := os.RemoveAll(filepath.Join(target, rel))
		if err != nil {
			t.Fatalf("remove %s: %v", rel, err)
		}
	}

	if manifest.Input != "" {
		copyTree(t, filepath.Join(fixtureDir, manifest.Input), target)
	}

	return target
}

// readManifest parses a fixture.yaml.
func readManifest(t *testing.T, path string) fixtureManifest {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest %s: %v", path, err)
	}

	var manifest fixtureManifest

	err = yaml.Unmarshal(data, &manifest)
	if err != nil {
		t.Fatalf("parse manifest %s: %v", path, err)
	}

	return manifest
}

// copyTree recursively copies src onto dst, preserving regular files and
// directories (the only entry kinds harness fixtures ship).
func copyTree(t *testing.T, src, dst string) {
	t.Helper()

	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read dir %s: %v", src, err)
	}

	err = os.MkdirAll(dst, 0o755)
	if err != nil {
		t.Fatalf("mkdir %s: %v", dst, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			copyTree(t, srcPath, dstPath)

			continue
		}

		copyFile(t, srcPath, dstPath)
	}
}

// copyFile copies one regular file.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()

	source, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer func() { _ = source.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, source)
	if err != nil {
		t.Fatalf("copy %s -> %s: %v", src, dst, err)
	}
}
