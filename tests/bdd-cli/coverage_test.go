//go:build bdd

package bddcli_test

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/scenarios"
)

const (
	templateExtension   = "extension"
	templateReplacement = "replacement"
)

// Hand-written, not generated: a generated version of these guards could be
// silenced by regenerating it. None brings up the harness, so all
// answer in well under a second.

// Every scenario the registry assigns this suite has exactly one
// generated test, in the file the registry names, and every generated
// test names a scenario that exists.
func TestScenarioCoverage(t *testing.T) {
	scenarios.CheckCoverage(t)
}

// Every fixture tree is named by a scenario. A tree nobody runs looks
// exactly like a tree that passes.
func TestFixtureTreesArePaired(t *testing.T) {
	scenarios.CheckFixtureTrees(t)
}

// Every fixture template overlay declares whether it extends the shipped
// template byte for byte or deliberately replaces it.
func TestFixtureTemplateOverlays(t *testing.T) {
	declarations := fixtureTemplateDeclarations()

	roots, err := filepath.Glob("fixtures/*/input/templates")
	if err != nil {
		t.Fatal(err)
	}

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() {
				return nil
			}

			assertTemplateOverlay(t, root, path, declarations)

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	for key := range declarations {
		t.Errorf("declared fixture template overlay is missing: %s", key)
	}
}

func fixtureTemplateDeclarations() map[string]string {
	return map[string]string{
		"engine-io-claude-writer/input/templates/build-code.fix-applier.prompt.tpl":        templateReplacement,
		"engine-io-claude-writer/input/templates/build-code.fix-applier.system.prompt.tpl": templateReplacement,

		"engine-io-events/input/templates/build-code.fix-applier.prompt.tpl":              templateReplacement,
		"engine-io-events/input/templates/build-code.fix-applier.system.prompt.tpl":       templateReplacement,
		"engine-io-cancel/input/templates/build-code.fix-applier.prompt.tpl":              templateReplacement,
		"engine-io-cancel/input/templates/build-code.fix-applier.system.prompt.tpl":       templateReplacement,
		"engine-io-cancel-codex/input/templates/build-code.fix-applier.prompt.tpl":        templateReplacement,
		"engine-io-cancel-codex/input/templates/build-code.fix-applier.system.prompt.tpl": templateReplacement,

		"build-code-no-test-files/input/templates/build-code.fix-applier.prompt.tpl":         templateExtension,
		"build-code-writable-writer/input/templates/build-code.fix-generator.prompt.tpl":     templateExtension,
		"build-code-writable-writer/input/templates/build-code.fix-applier.prompt.tpl":       templateExtension,
		"build-code-mixed-build/input/templates/build-code.fix-applier.prompt.tpl":           templateExtension,
		"build-code-spec-missing-registry/input/templates/build-code.fix-applier.prompt.tpl": templateExtension,

		"start-invalid-template/input/templates/build-code.fix-generator.prompt.tpl":                  templateReplacement,
		"init-existing/input/templates/build-code.fix-generator.prompt.tpl":                           templateReplacement,
		"writer-lifecycle-success/input/templates/build-code.fix-applier.prompt.tpl":                  templateReplacement,
		"writer-lifecycle-success/input/templates/build-code.fix-applier.system.prompt.tpl":           templateReplacement,
		"writer-lifecycle-failure/input/templates/build-code.fix-applier.prompt.tpl":                  templateReplacement,
		"writer-lifecycle-failure/input/templates/build-code.fix-applier.system.prompt.tpl":           templateReplacement,
		"writer-config-claude/input/templates/build-code.fix-applier.prompt.tpl":                      templateReplacement,
		"writer-config-claude/input/templates/build-code.fix-applier.system.prompt.tpl":               templateReplacement,
		"writer-config-codex/input/templates/build-code.fix-applier.prompt.tpl":                       templateReplacement,
		"writer-config-codex/input/templates/build-code.fix-applier.system.prompt.tpl":                templateReplacement,
		"build-code-go-package-does-not-compile/input/templates/build-code.fix-applier.prompt.tpl":    templateExtension,
		"build-code-jest-fix-named-test/input/templates/build-code.fix-applier.prompt.tpl":            templateExtension,
		"build-code-playwright-failing-assertion/input/templates/build-code.fix-applier.prompt.tpl":   templateExtension,
		"build-code-codex-missing-root/input/templates/build-code.fix-applier.prompt.tpl":             templateReplacement,
		"build-code-codex-missing-root/input/templates/build-code.fix-applier.system.prompt.tpl":      templateReplacement,
		"build-code-codex-selection/input/templates/build-code.fix-applier.prompt.tpl":                templateReplacement,
		"build-code-codex-selection/input/templates/build-code.fix-applier.system.prompt.tpl":         templateReplacement,
		"build-code-codex-nonzero/input/templates/build-code.fix-applier.prompt.tpl":                  templateReplacement,
		"build-code-codex-nonzero/input/templates/build-code.fix-applier.system.prompt.tpl":           templateReplacement,
		"build-code-writer-bound/input/templates/build-code.fix-applier.prompt.tpl":                   templateReplacement,
		"build-code-writer-bound/input/templates/build-code.fix-applier.system.prompt.tpl":            templateReplacement,
		"build-code-writer-empty/input/templates/build-code.fix-applier.prompt.tpl":                   templateReplacement,
		"build-code-writer-empty/input/templates/build-code.fix-applier.system.prompt.tpl":            templateReplacement,
		"build-code-writer-nonzero/input/templates/build-code.fix-applier.prompt.tpl":                 templateReplacement,
		"build-code-writer-nonzero/input/templates/build-code.fix-applier.system.prompt.tpl":          templateReplacement,
		"build-code-shrinking-bound/input/templates/build-code.fix-applier.system.prompt.tpl":         templateExtension,
		"build-code-no-confirmation-passes/input/templates/build-code.fix-applier.prompt.tpl":         templateReplacement,
		"build-code-no-confirmation-passes/input/templates/build-code.fix-applier.system.prompt.tpl":  templateReplacement,
		"build-code-no-confirmation-retries/input/templates/build-code.fix-applier.prompt.tpl":        templateReplacement,
		"build-code-no-confirmation-retries/input/templates/build-code.fix-applier.system.prompt.tpl": templateReplacement,
		"build-code-no-confirmation-bound/input/templates/build-code.fix-applier.prompt.tpl":          templateReplacement,
		"build-code-no-confirmation-bound/input/templates/build-code.fix-applier.system.prompt.tpl":   templateReplacement,
		"build-code-claude-replay/input/templates/build-code.fix-applier.prompt.tpl":                  templateReplacement,
		"build-code-claude-replay/input/templates/build-code.fix-applier.system.prompt.tpl":           templateReplacement,
		"build-code-claude-failure/input/templates/build-code.fix-applier.prompt.tpl":                 templateReplacement,
		"build-code-claude-failure/input/templates/build-code.fix-applier.system.prompt.tpl":          templateReplacement,
		"build-code-cross-walk/input/templates/build-code.fix-applier.prompt.tpl":                     templateExtension,
		"build-code-not-applied/input/templates/build-code.fix-applier.prompt.tpl":                    templateReplacement,
		"build-code-not-applied-bound/input/templates/build-code.fix-applier.prompt.tpl":              templateReplacement,
		"build-code-shrinking-bound/input/templates/build-code.fix-applier.prompt.tpl":                templateExtension,
		"build-code-missing-member/input/templates/build-code.fix-applier.prompt.tpl":                 templateExtension,
		"build-code-partial/input/templates/build-code.fix-applier.prompt.tpl":                        templateExtension,
	}
}

func assertTemplateOverlay(t *testing.T, root, path string, declarations map[string]string) {
	t.Helper()

	key, err := filepath.Rel("fixtures", path)
	if err != nil {
		t.Fatal(err)
	}

	key = filepath.ToSlash(key)
	switch declarations[key] {
	case templateExtension:
		template, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}

		assertTemplateExtension(t, path, filepath.Join("../../templates", template))
	case templateReplacement:
		// These overlays deliberately control writer behavior.
	default:
		t.Errorf("undeclared fixture template overlay: %s; declare extension or replacement", key)
	}

	delete(declarations, key)
}

func assertTemplateExtension(t *testing.T, overlay, shipped string) {
	t.Helper()

	actual, err := disk.Read(overlay)
	if err != nil {
		t.Fatal(err)
	}

	expected, err := disk.Read(shipped)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.HasPrefix(actual, expected) {
		t.Errorf("fixture template extension %s differs from shipped prefix %s", overlay, shipped)
	} else if len(actual) == len(expected) {
		t.Errorf("fixture template extension %s has no appended section", overlay)
	}
}
