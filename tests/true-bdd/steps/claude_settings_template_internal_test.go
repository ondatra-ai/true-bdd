package steps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"gopkg.in/yaml.v3"
)

func TestArchitectureClaudeTemplateMatchesScenario(t *testing.T) {
	t.Parallel()
	template := architectureClaudeTemplate(t)
	pattern := claudeSettingsScenarioPattern(t)

	for _, name := range []string{"plain", "a&b<c>", `quote"back\slash.+`, "single'quote"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}

			engine := filepath.Join(root, name, "true-bdd")

			err = disk.Write(engine, nil, disk.Private)
			if err != nil {
				t.Fatal(err)
			}

			invoked := filepath.Join(root, name, "engine-link")

			err = os.Symlink(engine, invoked)
			if err != nil {
				t.Fatal(err)
			}

			resolved, err := filepath.EvalSymlinks(engine)
			if err != nil {
				t.Fatal(err)
			}

			state := runAt(root)
			state.Harness = &Harness{BinPath: invoked}

			for _, executable := range []string{invoked, resolved} {
				settings := strings.ReplaceAll(template, "{guard_command}", jsonGuardCommand(t, executable))
				if !json.Valid([]byte(settings)) {
					t.Fatalf("substituted architecture template is invalid JSON: %s", settings)
				}

				err := disk.Write(filepath.Join(root, claudeSettingsFile), []byte(settings), disk.Private)
				if err != nil {
					t.Fatal(err)
				}

				err = assertEveryFileMatches(state, []string{claudeSettingsFile, filePatternMatches, pattern})
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func jsonGuardCommand(t *testing.T, executable string) string {
	t.Helper()

	command := "'" + strings.ReplaceAll(executable, "'", `'"'"'`) +
		"' claude-guard --policy 'tmp/run/claude-turn002-claude-config/policy.json'"

	encoded, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}

	return string(encoded[1 : len(encoded)-1])
}

func architectureClaudeTemplate(t *testing.T) string {
	t.Helper()

	data, err := disk.Read("../../../docs/architecture/architecture.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var document yaml.Node

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		t.Fatal(err)
	}

	note := claudeSettingsNote(&document)

	_, template, found := strings.Cut(note, "\n{\n")
	if !found {
		t.Fatal("architecture has no Claude settings template")
	}

	body, _, found := strings.Cut(template, "\n}\n")
	if !found {
		t.Fatal("architecture Claude settings template has no closing brace")
	}

	return "{\n" + body + "\n}\n"
}

func claudeSettingsNote(node *yaml.Node) string {
	if node.Kind == yaml.ScalarNode && strings.HasPrefix(node.Value, "Generated settings.json wire form") {
		return node.Value
	}

	for _, child := range node.Content {
		if note := claudeSettingsNote(child); note != "" {
			return note
		}
	}

	return ""
}

func claudeSettingsScenarioPattern(t *testing.T) string {
	t.Helper()

	scenarios, err := bddgo.LoadRegistry("../../../docs/scenarios.yaml")
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		if scenario.ID != "writer-config-002-claude" {
			continue
		}

		for _, step := range scenario.Steps {
			if strings.HasPrefix(step.Text, "every file matching ") {
				_, pattern, found := strings.Cut(step.Text, " matches ")
				if found {
					return pattern
				}
			}
		}
	}

	t.Fatal("Registry has no Claude settings pattern")

	return ""
}
