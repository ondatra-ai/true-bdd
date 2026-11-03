package steps

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

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

func assertChecklistQuestionAndFix(state *State, rule checklistPromptRule) error {
	literals, err := modelChecklistLiterals(state, rule.Checklist)
	if err != nil {
		return err
	}

	matches, err := modelPromptFiles(state, rule.Glob)
	if err != nil {
		return err
	}

	for _, path := range matches {
		content, err := disk.Read(path)
		if err != nil {
			return state.fail("read saved Model prompt: %w", err)
		}

		matched, err := matchesChecklistLiteral(content, literals, rule.Pattern)
		if err != nil {
			return state.fail("invalid Model expectation: %w", err)
		}

		if matched != (rule.Operator == modelPatternMatches) {
			return state.fail("prompt %s does not satisfy its %s checklist expectation", path, rule.Checklist)
		}
	}

	return nil
}

func matchesChecklistLiteral(content []byte, literals []checklistLiteral, pattern string) (bool, error) {
	for _, literal := range literals {
		expanded := strings.NewReplacer(
			"{{Q}}", regexp.QuoteMeta(literal.question),
			"{{rationale}}", regexp.QuoteMeta(literal.rationale),
			"{{F}}", regexp.QuoteMeta(literal.fix),
		).Replace(pattern)

		compiled, err := regexp.Compile(expanded)
		if err != nil {
			return false, fmt.Errorf("compile checklist expectation: %w", err)
		}

		if compiled.Match(content) {
			return true, nil
		}
	}

	return false, nil
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
