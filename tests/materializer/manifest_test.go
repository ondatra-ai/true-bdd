package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test-tree paths shared across the package's test files.
const (
	testPrdPath          = "docs/prd/prd.yaml"
	testEngineConfigPath = "true-bdd/true-bdd.yaml"
)

// writeFixture materializes a fixture dir: fixture.yaml with the given
// manifest text plus an input tree (path -> content). A nil inputFiles
// still creates the input/ dir so manifests declaring `input: input`
// stay loadable.
func writeFixture(t *testing.T, manifest string, inputFiles map[string]string) string {
	t.Helper()

	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "fixture.yaml"), []byte(manifest), 0o644)
	if err != nil {
		t.Fatalf("write fixture.yaml: %v", err)
	}

	err = os.MkdirAll(filepath.Join(dir, "input"), 0o755)
	if err != nil {
		t.Fatalf("mkdir input: %v", err)
	}

	for rel, content := range inputFiles {
		path := filepath.Join(dir, "input", rel)

		err = os.MkdirAll(filepath.Dir(path), 0o755)
		if err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}

		err = os.WriteFile(path, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	return dir
}

func TestLoadManifestMissingBase(t *testing.T) {
	dir := writeFixture(t, "input: input\n", nil)

	_, err := LoadManifest(dir)
	if !errors.Is(err, ErrBaseInvalid) {
		t.Fatalf("expected ErrBaseInvalid, got %v", err)
	}
}

func TestLoadManifestInvalidBase(t *testing.T) {
	dir := writeFixture(t, "base: full\n", nil)

	_, err := LoadManifest(dir)
	if !errors.Is(err, ErrBaseInvalid) {
		t.Fatalf("expected ErrBaseInvalid, got %v", err)
	}
}

func TestLoadManifestRejectsUnknownKeys(t *testing.T) {
	// A BDD-runner manifest copied by accident must fail loudly, not
	// have its cmd/expected silently ignored.
	dir := writeFixture(t, "base: none\ncmd: us refine 1.1\n", nil)

	_, err := LoadManifest(dir)
	if err == nil || !strings.Contains(err.Error(), "cmd") {
		t.Fatalf("expected unknown-key error naming cmd, got %v", err)
	}
}

func TestLoadManifestMissingInputDir(t *testing.T) {
	dir := writeFixture(t, "base: none\ninput: does-not-exist\n", nil)

	_, err := LoadManifest(dir)
	if !errors.Is(err, ErrInputDirMissing) {
		t.Fatalf("expected ErrInputDirMissing, got %v", err)
	}
}

func TestLoadManifestRemoveRequiresEngine(t *testing.T) {
	dir := writeFixture(t, "base: none\nremove:\n  - templates/x.tpl\n", nil)

	_, err := LoadManifest(dir)
	if !errors.Is(err, ErrRemoveRequiresEngine) {
		t.Fatalf("expected ErrRemoveRequiresEngine, got %v", err)
	}
}

func TestLoadManifestRemoveUnsafePaths(t *testing.T) {
	cases := []string{
		"\"../outside\"",
		"/etc/passwd",
		"\".\"",
		"\"a/../../b\"",
		"\"\"",
	}

	for _, entry := range cases {
		manifest := "base: engine\nremove:\n  - " + entry + "\n"
		dir := writeFixture(t, manifest, nil)

		_, err := LoadManifest(dir)
		if !errors.Is(err, ErrRemovePathUnsafe) {
			t.Fatalf("remove entry %s: expected ErrRemovePathUnsafe, got %v", entry, err)
		}
	}
}

func TestLoadManifestFilterRequiresEngine(t *testing.T) {
	manifest := "base: none\nchecklist_prompts:\n  us-refine:\n    - \"snippet\"\n"
	dir := writeFixture(t, manifest, nil)

	_, err := LoadManifest(dir)
	if !errors.Is(err, ErrFilterRequiresEngine) {
		t.Fatalf("expected ErrFilterRequiresEngine, got %v", err)
	}
}

func TestLoadManifestFilterDeclaredEmpty(t *testing.T) {
	dir := writeFixture(t, "base: engine\nchecklist_prompts:\n", nil)

	_, err := LoadManifest(dir)
	if !errors.Is(err, ErrFilterDeclaredEmpty) {
		t.Fatalf("expected ErrFilterDeclaredEmpty, got %v", err)
	}
}

func TestLoadManifestFilterEmptySnippetList(t *testing.T) {
	manifest := "base: engine\nchecklist_prompts:\n  us-refine: []\n"
	dir := writeFixture(t, manifest, nil)

	_, err := LoadManifest(dir)
	if !errors.Is(err, ErrFilterSnippetsEmpty) {
		t.Fatalf("expected ErrFilterSnippetsEmpty, got %v", err)
	}
}

func TestLoadManifestFilterBlankSnippet(t *testing.T) {
	manifest := "base: engine\nchecklist_prompts:\n  us-refine:\n    - \"   \"\n"
	dir := writeFixture(t, manifest, nil)

	_, err := LoadManifest(dir)
	if !errors.Is(err, ErrFilterSnippetsEmpty) {
		t.Fatalf("expected ErrFilterSnippetsEmpty, got %v", err)
	}
}

func TestLoadManifestFilterInputOverrideConflict(t *testing.T) {
	manifest := "base: engine\ninput: input\n" +
		"checklist_prompts:\n  us-refine:\n    - \"snippet\"\n"
	dir := writeFixture(t, manifest, map[string]string{
		"true-bdd/checklists/us-refine.yaml": "sections: []\n",
	})

	_, err := LoadManifest(dir)
	if !errors.Is(err, ErrFilterOverrideConflict) {
		t.Fatalf("expected ErrFilterOverrideConflict, got %v", err)
	}
}

func TestLoadManifestNormalizesCommands(t *testing.T) {
	manifest := "base: none\n" +
		"prep:\n  - \"  echo hi  \"\n  - \"   \"\n" +
		"teardown:\n  - \"\"\n  - \"docker compose down\"\n"
	dir := writeFixture(t, manifest, nil)

	loaded, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded.Prep) != 1 || loaded.Prep[0] != "echo hi" {
		t.Fatalf("prep not normalized: %#v", loaded.Prep)
	}

	if len(loaded.Teardown) != 1 || loaded.Teardown[0] != "docker compose down" {
		t.Fatalf("teardown not normalized: %#v", loaded.Teardown)
	}
}

func TestLoadManifestMinimalHappyPath(t *testing.T) {
	dir := writeFixture(t, "base: none\ninput: input\n", map[string]string{
		testPrdPath: "title: x\n",
	})

	loaded, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Base != BaseNone || loaded.Input != "input" {
		t.Fatalf("unexpected manifest: %#v", loaded)
	}
}
