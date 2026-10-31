//go:build bdd

package truebdd_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/tests/true-bdd/scenarios"
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
		"build-steps-critique-assertion/input/templates/build-steps.critique.system.prompt.tpl":    templateReplacement,
		"build-steps-critique-assertion/input/templates/build-steps.fix-applier.system.prompt.tpl": templateReplacement,
		"build-steps-critique-correct/input/templates/build-steps.critique.system.prompt.tpl":      templateReplacement,
		"build-steps-critique-correct/input/templates/build-steps.fix-applier.system.prompt.tpl":   templateReplacement,
		"build-steps-critique-operation/input/templates/build-steps.critique.system.prompt.tpl":    templateReplacement,
		"build-steps-critique-operation/input/templates/build-steps.fix-applier.system.prompt.tpl": templateReplacement,
		"build-steps-green/input/templates/build-steps.critique.system.prompt.tpl":                 templateReplacement,
		"build-steps-outside/input/templates/build-steps.fix-applier.prompt.tpl":                   templateExtension,
		"engine-io-claude-writer/input/templates/build-code.fix-applier.prompt.tpl":                templateReplacement,
		"engine-io-claude-writer/input/templates/build-code.fix-applier.system.prompt.tpl":         templateReplacement,

		"engine-io-events/input/templates/build-code.fix-applier.prompt.tpl":              templateReplacement,
		"engine-io-events/input/templates/build-code.fix-applier.system.prompt.tpl":       templateReplacement,
		"engine-io-cancel/input/templates/build-code.fix-applier.prompt.tpl":              templateReplacement,
		"build-steps-green/input/templates/build-steps.fix-applier.system.prompt.tpl":     templateReplacement,
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

		"concept-link-unquoted-confirmation/input/templates/us-checklist.fix-applier.prompt.tpl": templateExtension,
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

// No document a fixture's model turns read names a Scenario beside an
// outcome word in a comment: the link, binding and word Prompts embed it
// whole, and a model told the verdict records it as its own (#259).
func TestFixtureCommentsNameNoVerdict(t *testing.T) {
	rule := newVerdictRule()

	recordings, err := filepath.Glob("fixtures/*/cassettes")
	if err != nil {
		t.Fatal(err)
	}

	for _, cassettes := range recordings {
		if !makesModelTurn(t, cassettes) {
			continue
		}

		for _, root := range []string{"input/docs", "input/true-bdd/checklists"} {
			assertNoVerdictComments(t, filepath.Join(filepath.Dir(cassettes), root), rule)
		}
	}
}

// verdictRule is what the guard refuses: one comment naming a Scenario id
// (E2E-001, or a lineage key such as 95.1-001-mcp-initialize) and an outcome
// word. A comment with only one of the two passes.
type verdictRule struct {
	scenarioID *regexp.Regexp
	outcome    *regexp.Regexp
}

func newVerdictRule() verdictRule {
	return verdictRule{
		scenarioID: regexp.MustCompile(`\b(?:E2E-\d+|[A-Za-z0-9][A-Za-z0-9.-]*-\d{3})\b`),
		outcome: regexp.MustCompile(
			`(?i)\b(?:pass(?:es|ed)?|fail(?:s|ed)?|falls? under|defect(?:s|ive)?|undefined|unmapped|cover(?:s|ed)?)\b`),
	}
}

func (r verdictRule) refuses(comment string) bool {
	return r.scenarioID.MatchString(comment) && r.outcome.MatchString(comment)
}

// makesModelTurn reports whether a fixture's recordings hold a claude or
// codex turn, its services' or its judge's.
func makesModelTurn(t *testing.T, cassettes string) bool {
	t.Helper()

	turn := regexp.MustCompile(`^(?:claude|codex)-\d+$`)
	found := false

	err := filepath.WalkDir(cassettes, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() && turn.MatchString(entry.Name()) {
			found = true

			return fs.SkipAll
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return found
}

func assertNoVerdictComments(t *testing.T, root string, rule verdictRule) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		extension := filepath.Ext(path)
		if entry.IsDir() || (extension != ".yaml" && extension != ".yml" && extension != ".md") {
			return nil
		}

		for _, comment := range documentComments(t, path) {
			if rule.refuses(comment) {
				t.Errorf("%s: a comment names a Scenario beside an outcome, which tells a model reading it "+
					"the verdict; keep the note in bdd-harness.md or prep.sh:\n%s", path, strings.TrimSpace(comment))
			}
		}

		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}

// documentComments returns each comment block: Markdown's HTML comments,
// YAML's head, line and foot comments, or, where a fixture's YAML is meant
// not to parse, its runs of whole-line comments.
func documentComments(t *testing.T, path string) []string {
	t.Helper()

	data, err := disk.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Ext(path) == ".md" {
		matches := regexp.MustCompile(`(?s)<!--(.*?)-->`).FindAllSubmatch(data, -1)

		comments := make([]string, 0, len(matches))
		for _, match := range matches {
			comments = append(comments, string(match[1]))
		}

		return comments
	}

	comments, err := yamlComments(data)
	if err != nil {
		return wholeLineComments(data)
	}

	return comments
}

func yamlComments(data []byte) ([]string, error) {
	var comments []string

	decoder := yaml.NewDecoder(bytes.NewReader(data))

	for {
		var node yaml.Node

		err := decoder.Decode(&node)
		if errors.Is(err, io.EOF) {
			return comments, nil
		}

		if err != nil {
			return nil, fmt.Errorf("parsing YAML: %w", err)
		}

		comments = appendNodeComments(comments, &node)
	}
}

func appendNodeComments(comments []string, node *yaml.Node) []string {
	for _, comment := range []string{node.HeadComment, node.LineComment, node.FootComment} {
		if comment != "" {
			comments = append(comments, comment)
		}
	}

	for _, child := range node.Content {
		comments = appendNodeComments(comments, child)
	}

	return comments
}

func wholeLineComments(data []byte) []string {
	var (
		comments []string
		block    []string
	)

	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			block = append(block, line)

			continue
		}

		if len(block) > 0 {
			comments = append(comments, strings.Join(block, "\n"))
			block = nil
		}
	}

	if len(block) > 0 {
		comments = append(comments, strings.Join(block, "\n"))
	}

	return comments
}
