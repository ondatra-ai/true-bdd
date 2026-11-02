package steps

import (
	"bytes"
	"path/filepath"
	"regexp"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
	"gopkg.in/yaml.v3"
)

const checklistPairSize = 2

type checklistLiteral struct {
	question  string
	rationale string
	fix       string
}

func assertChecklistQuestionAndFix(state *State, model string) error {
	paths := map[string]struct{ stem, glob string }{
		"us_refine_checklist":  {"us-refine", modelStoryPromptPattern},
		"us_apply_checklist":   {"us-apply", "tmp/*/01-mcp-endpoint-002-get-refused-checklist-*-user.txt"},
		"scen_check_checklist": {"scen-check", modelScenarioPromptPattern},
	}

	selected, ok := paths[model]
	if !ok {
		return state.fail("no checklist for Model %q", model)
	}

	literals, err := modelChecklistLiterals(state, selected.stem)
	if err != nil {
		return err
	}

	matches, err := modelPromptFiles(state, selected.glob)
	if err != nil {
		return err
	}

	for _, path := range matches {
		content, err := disk.Read(path)
		if err != nil {
			return state.fail("read saved Model prompt: %w", err)
		}

		if !matchesChecklistLiteral(content, literals) {
			return state.fail("%s prompt does not hold its checklist's question and fix verbatim", model)
		}
	}

	return nil
}

func matchesChecklistLiteral(content []byte, literals []checklistLiteral) bool {
	for _, literal := range literals {
		question := regexp.QuoteMeta(literal.question)
		if literal.rationale != "" {
			question += regexp.QuoteMeta(" SO THAT WE ENSURE " + literal.rationale)
		}

		pattern := "## Validation Question\n\n" + question + "\n\n## If Validation Fails\n(?s:.*?)\n\n" +
			regexp.QuoteMeta(literal.fix) + "\n\n## Answer Format"
		if regexp.MustCompile(pattern).Match(content) {
			return true
		}
	}

	return false
}

func modelChecklistLiterals(state *State, stem string) ([]checklistLiteral, error) {
	configuration, err := state.containedPath("true-bdd/true-bdd.yaml")
	if err != nil {
		return nil, err
	}

	content, err := disk.Read(configuration)
	if err != nil {
		return nil, state.fail("read checklist configuration: %w", err)
	}

	var cfg struct {
		Paths struct {
			ChecklistsDir string `yaml:"checklists_dir"`
		} `yaml:"paths"`
	}

	err = yaml.Unmarshal(content, &cfg)
	if err != nil {
		return nil, state.fail("read checklist path: %w", err)
	}

	path, err := state.containedPath(filepath.Join(cfg.Paths.ChecklistsDir, stem+".yaml"))
	if err != nil {
		return nil, err
	}

	content, err = disk.Read(path)
	if err != nil {
		return nil, state.fail("read Model's checklist: %w", err)
	}

	var document yaml.Node

	err = yaml.Unmarshal(content, &document)
	if err != nil {
		return nil, state.fail("decode Model's checklist: %w", err)
	}

	literals := checklistLiterals(&document)
	if len(literals) == 0 {
		return nil, state.fail("checklist %s contains no question and fix", stem)
	}

	return literals, nil
}

func checklistLiterals(node *yaml.Node) []checklistLiteral {
	var literals []checklistLiteral

	if node.Kind == yaml.MappingNode {
		var literal checklistLiteral

		for index := 0; index < len(node.Content); index += checklistPairSize {
			switch node.Content[index].Value {
			case "Q":
				literal.question = node.Content[index+1].Value
			case "F":
				literal.fix = node.Content[index+1].Value
			case "rationale":
				literal.rationale = node.Content[index+1].Value
			}
		}

		if literal.question != "" && literal.fix != "" {
			literals = append(literals, literal)
		}
	}

	for _, child := range node.Content {
		literals = append(literals, checklistLiterals(child)...)
	}

	return literals
}

func modelPromptFiles(state *State, pattern string) ([]string, error) {
	if state.Result == nil {
		return nil, state.fail("%w", ErrNoRun)
	}

	glob, err := state.containedPath(pattern)
	if err != nil {
		return nil, err
	}

	paths, err := filepath.Glob(glob)
	if err != nil {
		return nil, state.fail("find saved Model prompt: %w", err)
	}

	if len(paths) == 0 {
		return nil, state.fail("no saved Model prompt matches %s", pattern)
	}

	for index, path := range paths {
		paths[index], err = state.containedMatch(pattern, path)
		if err != nil {
			return nil, err
		}
	}

	return paths, nil
}

//truebdd:concept then "file content"
func assertModelSchemeShipped(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	relative := filepath.Join("true-bdd/models", args[0]+".schema.yaml")

	path, err := state.containedPath(relative)
	if err != nil {
		return err
	}

	actual, err := disk.Read(path)
	if err != nil {
		return state.fail("read shipped Model scheme: %w", err)
	}

	root, err := runner.FindRepoRoot()
	if err != nil {
		return state.fail("find Model's Specification: %w", err)
	}

	expected, err := disk.Read(filepath.Join(root, relative))
	if err != nil {
		return state.fail("read Model's declared scheme: %w", err)
	}

	if !bytes.Equal(actual, expected) {
		return state.fail("shipped %s scheme differs from its declaration", args[0])
	}

	return nil
}
