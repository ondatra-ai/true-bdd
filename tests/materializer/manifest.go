package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Base layer values accepted by the manifest.
const (
	// BaseEngine overlays the live engine layer (`true-bdd/` +
	// `templates/` from the repo root) before the fixture input.
	BaseEngine = "engine"
	// BaseNone starts from an empty target — bare-folder fixtures
	// (e.g. the P1 "missing true-bdd.yaml" scenario).
	BaseNone = "none"
)

// ErrBaseInvalid is returned when `base` is missing or not one of
// engine|none. The field is required so a fixture's layering is always
// explicit.
var ErrBaseInvalid = errors.New(
	"fixture.yaml: base is required and must be \"engine\" or \"none\"")

// ErrInputDirMissing is returned when the manifest declares an input
// directory that does not exist under the fixture dir.
var ErrInputDirMissing = errors.New("fixture.yaml: declared input directory does not exist")

// ErrRemoveRequiresEngine is returned when `remove` is declared with
// `base: none` — there is no base layer to delete from.
var ErrRemoveRequiresEngine = errors.New(
	"fixture.yaml: remove requires base: engine — with base: none there is nothing to remove")

// ErrRemovePathUnsafe is returned for remove entries that are empty,
// absolute, or escape the target directory.
var ErrRemovePathUnsafe = errors.New(
	"fixture.yaml: remove paths must be clean, relative, and inside the target")

// ErrFilterRequiresEngine is returned when checklist_prompts is
// declared with `base: none` — the filter's whole point is deriving
// from the LIVE engine checklist, and an input-shipped checklist would
// be a conflict anyway.
var ErrFilterRequiresEngine = errors.New(
	"fixture.yaml: checklist_prompts requires base: engine — filtering derives from the live checklist")

// ErrFilterDeclaredEmpty is returned when the checklist_prompts KEY is
// present but null/empty: a declared filter that selects nothing would
// silently run the full checklist.
var ErrFilterDeclaredEmpty = errors.New(
	"fixture.yaml: checklist_prompts declared but empty — remove the key or declare a selection")

// ErrFilterSnippetsEmpty is returned for a stem with an empty or
// all-blank snippet list.
var ErrFilterSnippetsEmpty = errors.New(
	"fixture.yaml: checklist_prompts stem has an empty snippet list or a blank snippet")

// ErrFilterOverrideConflict is returned when a fixture both declares
// checklist_prompts for a stem and ships an input override for the
// same checklist file — two competing sources of truth.
var ErrFilterOverrideConflict = errors.New(
	"fixture.yaml: checklist_prompts declared but the input tree also overrides the same checklist")

// Manifest is the on-disk shape of a harness fixture's fixture.yaml.
// Unlike the BDD runner's manifest there is no cmd/answers/expected:
// the Playwright test dispatches commands through the harness UI and
// owns every assertion.
type Manifest struct {
	// Base selects the starting layer: "engine" or "none". Required.
	Base string `yaml:"base"`

	// Input is the path (relative to the fixture dir) of the directory
	// tree overlaid onto the target AFTER the base layer and remove
	// step. Optional — bare-folder fixtures may omit it.
	Input string `yaml:"input,omitempty"`

	// Prep is a list of shell commands run in the target via `bash -c`
	// BEFORE the baseline tree hash, so their side effects never show
	// up in a test's mutation diff. Optional.
	Prep []string `yaml:"prep,omitempty"`

	// Teardown is a list of shell commands the materializer validates
	// and echoes back in its JSON output; the TypeScript harness runs
	// them in test-scoped teardown. The materializer itself NEVER runs
	// them. Optional.
	Teardown []string `yaml:"teardown,omitempty"`

	// ChecklistPrompts maps a checklist stem (e.g. "us-refine") to
	// snippet strings; the target's live checklist is rewritten to
	// contain only the matched prompts. Semantics are shared with
	// tests/bdd-cli/runner (whitespace-collapsed substring, exactly one
	// live non-skipped prompt per snippet). Optional; requires
	// base: engine.
	ChecklistPrompts map[string][]string `yaml:"checklist_prompts,omitempty"`

	// Remove lists target-relative paths deleted AFTER the base
	// overlay and BEFORE the input overlay — used to engineer
	// missing-checklist / missing-dir cases from the engine base.
	// Every listed path must exist at removal time. Optional; requires
	// base: engine.
	Remove []string `yaml:"remove,omitempty"`
}

// LoadManifest reads and validates <fixtureDir>/fixture.yaml. All
// validation that does not need the materialized tree happens here so
// a broken fixture fails before any copying starts.
func LoadManifest(fixtureDir string) (*Manifest, error) {
	path := filepath.Join(fixtureDir, "fixture.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture.yaml: %w", err)
	}

	manifest, err := decodeManifestStrict(data)
	if err != nil {
		return nil, err
	}

	err = validateManifest(manifest, data, fixtureDir)
	if err != nil {
		return nil, err
	}

	manifest.Prep = normalizeCommands(manifest.Prep)
	manifest.Teardown = normalizeCommands(manifest.Teardown)

	return manifest, nil
}

// decodeManifestStrict decodes fixture.yaml rejecting unknown keys, so
// a stray `cmd:`/`expected:` copied from a BDD fixture — or a typoed
// field — fails loudly instead of being silently ignored.
func decodeManifestStrict(data []byte) (*Manifest, error) {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)

	var manifest Manifest

	err := decoder.Decode(&manifest)
	if err != nil {
		return nil, fmt.Errorf("parse fixture.yaml: %w", err)
	}

	return &manifest, nil
}

func validateManifest(manifest *Manifest, raw []byte, fixtureDir string) error {
	if manifest.Base != BaseEngine && manifest.Base != BaseNone {
		return fmt.Errorf("%w: got %q", ErrBaseInvalid, manifest.Base)
	}

	err := validateInput(manifest, fixtureDir)
	if err != nil {
		return err
	}

	err = validateRemove(manifest)
	if err != nil {
		return err
	}

	return validateChecklistPrompts(manifest, raw, fixtureDir)
}

func validateInput(manifest *Manifest, fixtureDir string) error {
	manifest.Input = strings.TrimSpace(manifest.Input)
	if manifest.Input == "" {
		return nil
	}

	inputDir := filepath.Join(fixtureDir, manifest.Input)

	info, err := os.Stat(inputDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrInputDirMissing, inputDir)
	}

	return nil
}

func validateRemove(manifest *Manifest) error {
	if len(manifest.Remove) == 0 {
		return nil
	}

	if manifest.Base != BaseEngine {
		return ErrRemoveRequiresEngine
	}

	for _, raw := range manifest.Remove {
		trimmed := strings.TrimSpace(raw)
		cleaned := filepath.Clean(trimmed)

		unsafe := trimmed == "" ||
			filepath.IsAbs(cleaned) ||
			cleaned == "." ||
			cleaned == ".." ||
			strings.HasPrefix(cleaned, "../")
		if unsafe {
			return fmt.Errorf("%w: %q", ErrRemovePathUnsafe, raw)
		}
	}

	return nil
}

// validateChecklistPrompts checks everything about the filter
// declaration that does not require the materialized tree: base
// compatibility, non-empty declaration, non-blank snippets, and the
// input-override conflict. Snippet↔prompt matching happens later in
// Materialize via runner.FilterChecklistFile against the target tree.
func validateChecklistPrompts(manifest *Manifest, raw []byte, fixtureDir string) error {
	if declaredEmptyChecklistPrompts(raw, manifest) {
		return ErrFilterDeclaredEmpty
	}

	if len(manifest.ChecklistPrompts) == 0 {
		return nil
	}

	if manifest.Base != BaseEngine {
		return ErrFilterRequiresEngine
	}

	for stem, snippets := range manifest.ChecklistPrompts {
		if len(snippets) == 0 {
			return fmt.Errorf("%w: stem %s", ErrFilterSnippetsEmpty, stem)
		}

		for _, snippet := range snippets {
			if strings.TrimSpace(snippet) == "" {
				return fmt.Errorf("%w: stem %s", ErrFilterSnippetsEmpty, stem)
			}
		}

		err := rejectInputOverride(manifest, fixtureDir, stem)
		if err != nil {
			return err
		}
	}

	return nil
}

func rejectInputOverride(manifest *Manifest, fixtureDir, stem string) error {
	if manifest.Input == "" {
		return nil
	}

	override := filepath.Join(fixtureDir, manifest.Input,
		"true-bdd", "checklists", stem+".yaml")

	_, err := os.Stat(override)
	if err == nil {
		return fmt.Errorf("%w: %s", ErrFilterOverrideConflict, override)
	}

	return nil
}

// declaredEmptyChecklistPrompts reports whether the checklist_prompts
// KEY is present in the raw YAML while the decoded map is empty —
// mirroring the BDD runner's rejectEmptyFilterDeclaration.
func declaredEmptyChecklistPrompts(raw []byte, manifest *Manifest) bool {
	if len(manifest.ChecklistPrompts) > 0 {
		return false
	}

	var doc yaml.Node

	err := yaml.Unmarshal(raw, &doc)
	if err != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return false
	}

	root := doc.Content[0]
	for idx := 0; idx+1 < len(root.Content); idx += 2 {
		if root.Content[idx].Value == "checklist_prompts" {
			return true
		}
	}

	return false
}

// normalizeCommands trims each command and drops blank entries,
// matching the BDD runner's tolerance for stray empty lines.
func normalizeCommands(cmds []string) []string {
	out := make([]string, 0, len(cmds))

	for _, raw := range cmds {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		out = append(out, trimmed)
	}

	return out
}
