package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
	"gopkg.in/yaml.v3"
)

// testChecklist has two live prompts and one skipped prompt, so tests
// can exercise every snippet-resolution branch.
const testChecklist = `version: "1.0"
sections:
  - id: quality
    name: "Quality"
    validation_prompts:
      - Q: |
          First live prompt about apples.
        rationale: "r1"
      - Q: |
          Second live prompt about bananas.
        rationale: "r2"
      - Q: |
          Retired prompt about cherries.
        skip: "superseded"
`

// makeRepoRoot builds a synthetic engine layer: true-bdd/ (config +
// one checklist) and templates/ — the exact subtrees runner.RepoLayer
// declares.
func makeRepoRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	files := map[string]string{
		testEngineConfigPath:                 "engine:\n  model: \"test-model\"\n",
		"true-bdd/checklists/us-refine.yaml": testChecklist,
		"templates/us-checklist.prompt.tpl":  "template body\n",
	}

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

	return root
}

func materializeInto(t *testing.T, fixtureDir, repoRoot string) (*Result, string, error) {
	t.Helper()

	target := filepath.Join(t.TempDir(), "target")

	res, err := Materialize(Options{
		FixtureDir: fixtureDir,
		TargetDir:  target,
		RepoRoot:   repoRoot,
	})

	return res, target, err
}

func readTargetFile(t *testing.T, target, rel string) string {
	t.Helper()

	data, err := disk.Read(filepath.Join(target, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}

	return string(data)
}

func TestMaterializeBaseNoneIsInputOnly(t *testing.T) {
	dir := writeFixture(t, "base: none\ninput: input\n", map[string]string{
		testProductPath: "title: bare\n",
	})

	res, target, err := materializeInto(t, dir, makeRepoRoot(t))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	if got := readTargetFile(t, target, "docs/product/product.yaml"); got != "title: bare\n" {
		t.Fatalf("input file content = %q", got)
	}

	_, statErr := os.Stat(filepath.Join(target, "true-bdd"))
	if !os.IsNotExist(statErr) {
		t.Fatalf("base:none must not copy the engine layer, stat err = %v", statErr)
	}

	if res.Base != BaseNone {
		t.Fatalf("result base = %q", res.Base)
	}
}

func TestMaterializeOverlayPrecedence(t *testing.T) {
	dir := writeFixture(t, "base: engine\ninput: input\n", map[string]string{
		testEngineConfigPath: "engine:\n  model: \"fixture-override\"\n",
	})

	_, target, err := materializeInto(t, dir, makeRepoRoot(t))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	got := readTargetFile(t, target, "true-bdd/true-bdd.yaml")
	if !strings.Contains(got, "fixture-override") {
		t.Fatalf("input must win over the engine layer, got %q", got)
	}

	// Untouched engine files survive the overlay.
	tpl := readTargetFile(t, target, "templates/us-checklist.prompt.tpl")
	if tpl != "template body\n" {
		t.Fatalf("engine template mangled: %q", tpl)
	}
}

func TestMaterializeRemoveFileAndDir(t *testing.T) {
	manifest := "base: engine\nremove:\n" +
		"  - templates/us-checklist.prompt.tpl\n" +
		"  - true-bdd/checklists\n"
	dir := writeFixture(t, manifest, nil)

	_, target, err := materializeInto(t, dir, makeRepoRoot(t))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	for _, rel := range []string{"templates/us-checklist.prompt.tpl", "true-bdd/checklists"} {
		_, statErr := os.Stat(filepath.Join(target, rel))
		if !os.IsNotExist(statErr) {
			t.Fatalf("%s should be removed, stat err = %v", rel, statErr)
		}
	}
}

func TestMaterializeRemoveMissingPathFails(t *testing.T) {
	dir := writeFixture(t, "base: engine\nremove:\n  - templates/typo.tpl\n", nil)

	_, _, err := materializeInto(t, dir, makeRepoRoot(t))
	if !errors.Is(err, ErrRemovePathMissing) {
		t.Fatalf("expected ErrRemovePathMissing, got %v", err)
	}
}

func TestMaterializeRemoveRunsBeforeInputOverlay(t *testing.T) {
	// remove deletes the engine copy; the input overlay then supplies
	// a replacement — pinning the documented base → remove → input
	// ordering.
	manifest := "base: engine\ninput: input\nremove:\n  - true-bdd/true-bdd.yaml\n"
	dir := writeFixture(t, manifest, map[string]string{
		testEngineConfigPath: "engine:\n  model: \"restored\"\n",
	})

	_, target, err := materializeInto(t, dir, makeRepoRoot(t))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	got := readTargetFile(t, target, "true-bdd/true-bdd.yaml")
	if !strings.Contains(got, "restored") {
		t.Fatalf("input replacement missing, got %q", got)
	}
}

// filteredChecklist is the minimal shape needed to inspect a generated
// checklist file.
type filteredChecklist struct {
	Sections []struct {
		ValidationPrompts []struct {
			Q string `yaml:"Q"` //nolint:tagliatelle // checklist schema key
		} `yaml:"validation_prompts"`
	} `yaml:"sections"`
}

func TestMaterializeChecklistFilterHappyPath(t *testing.T) {
	manifest := "base: engine\nchecklist_prompts:\n  us-refine:\n    - \"about apples\"\n"
	dir := writeFixture(t, manifest, nil)

	res, target, err := materializeInto(t, dir, makeRepoRoot(t))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	raw := readTargetFile(t, target, "true-bdd/checklists/us-refine.yaml")

	var parsed filteredChecklist

	err = yaml.Unmarshal([]byte(raw), &parsed)
	if err != nil {
		t.Fatalf("parse filtered checklist: %v", err)
	}

	if len(parsed.Sections) != 1 || len(parsed.Sections[0].ValidationPrompts) != 1 {
		t.Fatalf("expected exactly one surviving prompt, got %s", raw)
	}

	if !strings.Contains(parsed.Sections[0].ValidationPrompts[0].Q, "apples") {
		t.Fatalf("wrong prompt survived: %s", raw)
	}

	// The baseline hashes the FILTERED file, so a test-time recompute
	// of the unchanged file matches the baseline.
	sum := sha256.Sum256([]byte(raw))
	if res.Baseline["true-bdd/checklists/us-refine.yaml"] != hex.EncodeToString(sum[:]) {
		t.Fatal("baseline hash does not match the filtered checklist content")
	}
}

func TestMaterializeChecklistFilterFailureModes(t *testing.T) {
	cases := []struct {
		name    string
		snippet string
		want    error
	}{
		{"unmatched snippet", "about dragonfruit", runner.ErrSnippetUnmatched},
		{"ambiguous snippet", "live prompt", runner.ErrSnippetAmbiguous},
		{"skipped prompt not selectable", "about cherries", runner.ErrSnippetUnmatched},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			manifest := "base: engine\nchecklist_prompts:\n  us-refine:\n" +
				"    - \"" + testCase.snippet + "\"\n"
			dir := writeFixture(t, manifest, nil)

			_, _, err := materializeInto(t, dir, makeRepoRoot(t))
			if !errors.Is(err, testCase.want) {
				t.Fatalf("expected %v, got %v", testCase.want, err)
			}
		})
	}
}

func TestMaterializeChecklistFilterDuplicateResolution(t *testing.T) {
	manifest := "base: engine\nchecklist_prompts:\n  us-refine:\n" +
		"    - \"about apples\"\n    - \"First live\"\n"
	dir := writeFixture(t, manifest, nil)

	_, _, err := materializeInto(t, dir, makeRepoRoot(t))
	if !errors.Is(err, runner.ErrSnippetDuplicate) {
		t.Fatalf("expected ErrSnippetDuplicate, got %v", err)
	}
}

func TestMaterializeChecklistStemUnknown(t *testing.T) {
	manifest := "base: engine\nchecklist_prompts:\n  no-such-checklist:\n    - \"x\"\n"
	dir := writeFixture(t, manifest, nil)

	_, _, err := materializeInto(t, dir, makeRepoRoot(t))
	if !errors.Is(err, ErrChecklistStemUnknown) {
		t.Fatalf("expected ErrChecklistStemUnknown, got %v", err)
	}
}

func TestMaterializeChecklistRemovedThenFiltered(t *testing.T) {
	manifest := "base: engine\n" +
		"remove:\n  - true-bdd/checklists/us-refine.yaml\n" +
		"checklist_prompts:\n  us-refine:\n    - \"about apples\"\n"
	dir := writeFixture(t, manifest, nil)

	_, _, err := materializeInto(t, dir, makeRepoRoot(t))
	if !errors.Is(err, ErrChecklistStemUnknown) {
		t.Fatalf("expected ErrChecklistStemUnknown, got %v", err)
	}
}

func TestMaterializeTargetMustBeEmpty(t *testing.T) {
	dir := writeFixture(t, "base: none\n", nil)
	target := t.TempDir()

	err := os.WriteFile(filepath.Join(target, "stale.txt"), []byte("x"), 0o644)
	if err != nil {
		t.Fatalf("seed stale file: %v", err)
	}

	_, err = Materialize(Options{
		FixtureDir: dir, TargetDir: target, RepoRoot: makeRepoRoot(t),
	})
	if !errors.Is(err, ErrTargetNotEmpty) {
		t.Fatalf("expected ErrTargetNotEmpty, got %v", err)
	}
}

func TestMaterializeEngineRequiresRepoRoot(t *testing.T) {
	dir := writeFixture(t, "base: engine\n", nil)

	_, _, err := materializeInto(t, dir, "")
	if !errors.Is(err, ErrRepoRootRequired) {
		t.Fatalf("expected ErrRepoRootRequired, got %v", err)
	}
}

func TestMaterializePrepRunsBeforeBaseline(t *testing.T) {
	manifest := "base: none\nprep:\n  - \"printf made-by-prep > prep-artifact.txt\"\n"
	dir := writeFixture(t, manifest, nil)

	res, target, err := materializeInto(t, dir, makeRepoRoot(t))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	if got := readTargetFile(t, target, "prep-artifact.txt"); got != "made-by-prep" {
		t.Fatalf("prep artifact content = %q", got)
	}

	sum := sha256.Sum256([]byte("made-by-prep"))
	if res.Baseline["prep-artifact.txt"] != hex.EncodeToString(sum[:]) {
		t.Fatal("prep side effects must be inside the baseline hash")
	}
}

func TestMaterializePrepFailureAborts(t *testing.T) {
	dir := writeFixture(t, "base: none\nprep:\n  - \"exit 3\"\n", nil)

	_, _, err := materializeInto(t, dir, makeRepoRoot(t))
	if err == nil || !strings.Contains(err.Error(), "prep[0]") {
		t.Fatalf("expected prep failure, got %v", err)
	}
}

func TestMaterializeBaselineExcludesTmp(t *testing.T) {
	dir := writeFixture(t, "base: none\ninput: input\n", map[string]string{
		"tmp/scratch.txt": "runtime noise",
		testProductPath:   "title: x\n",
	})

	res, target, err := materializeInto(t, dir, makeRepoRoot(t))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	if _, present := res.Baseline["tmp/scratch.txt"]; present {
		t.Fatal("tmp/** must be excluded from the baseline")
	}

	if _, present := res.Baseline[testProductPath]; !present {
		t.Fatal("canonical file missing from baseline")
	}

	// Excluded from the hash, but still materialized on disk.
	if got := readTargetFile(t, target, "tmp/scratch.txt"); got != "runtime noise" {
		t.Fatalf("tmp file content = %q", got)
	}
}

func TestMaterializeTeardownEchoedNotExecuted(t *testing.T) {
	manifest := "base: none\nteardown:\n  - \"touch teardown-ran.txt\"\n"
	dir := writeFixture(t, manifest, nil)

	res, target, err := materializeInto(t, dir, makeRepoRoot(t))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	if len(res.Teardown) != 1 || res.Teardown[0] != "touch teardown-ran.txt" {
		t.Fatalf("teardown not echoed: %#v", res.Teardown)
	}

	_, statErr := os.Stat(filepath.Join(target, "teardown-ran.txt"))
	if !os.IsNotExist(statErr) {
		t.Fatalf("teardown must never run during materialization, stat err = %v", statErr)
	}
}
