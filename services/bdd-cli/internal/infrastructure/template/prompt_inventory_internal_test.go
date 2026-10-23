package template

import (
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"text/template"
	"text/template/parse"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

const inventoryRoot = "../../../../../"

type promptDescription struct {
	Fields    map[string]string `yaml:"fields"`
	Rendering string            `yaml:"rendering"`
}

func readInventoryYAML(t *testing.T, path string, target any) {
	t.Helper()

	content, err := disk.Read(filepath.Join(inventoryRoot, path))
	if err != nil {
		t.Fatal(err)
	}

	err = yaml.Unmarshal(content, target)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPromptDataInventory(t *testing.T) {
	t.Parallel()

	var (
		config struct {
			Templates struct {
				Prompts map[string]string `yaml:"prompts"`
			} `yaml:"templates"`
		}
		architecture struct {
			Architecture struct {
				PromptData map[string]promptDescription `yaml:"prompt_data"`
			} `yaml:"architecture"`
		}
	)

	readInventoryYAML(t, "true-bdd/true-bdd.yaml", &config)
	readInventoryYAML(t, "docs/architecture/architecture.yaml", &architecture)

	for key, path := range config.Templates.Prompts {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			description, exists := architecture.Architecture.PromptData[key]

			content, err := disk.Read(filepath.Join(inventoryRoot, path))
			if err != nil {
				t.Fatal(err)
			}

			actual := templateFields(t, string(content))
			if !exists {
				t.Fatalf("missing template key; reads: %v", slices.Sorted(maps.Keys(actual)))
			}

			comparePromptFields(t, actual, description.Fields)
		})
	}

	for key := range architecture.Architecture.PromptData {
		if _, ok := config.Templates.Prompts[key]; !ok {
			t.Errorf("prompt_data describes unused template %q", key)
		}
	}

	paths, err := filepath.Glob(filepath.Join(inventoryRoot, "templates", "*"))
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range paths {
		if !slices.ContainsFunc(slices.Collect(maps.Values(config.Templates.Prompts)), func(configured string) bool {
			return filepath.Clean(filepath.Join(inventoryRoot, configured)) == path
		}) {
			t.Errorf("template has no configured key: %s", path)
		}
	}
}

func comparePromptFields(t *testing.T, actual map[string]bool, described map[string]string) {
	t.Helper()

	for field := range actual {
		if strings.TrimSpace(described[field]) == "" {
			t.Errorf("template reads undocumented field %s", field)
		}
	}

	for field := range described {
		if !actual[field] {
			t.Errorf("architecture describes unread field %s", field)
		}
	}
}

func templateFields(t *testing.T, source string) map[string]bool {
	t.Helper()

	parsed, err := template.New("prompt").Funcs(getCommonTemplateFunctions()).Parse(source)
	if err != nil {
		t.Fatal(err)
	}

	fields := make(map[string]bool)
	scope := fieldScope{dot: "", variables: map[string]string{"$": ""}}
	walkFields(t, parsed.Root, scope, fields)

	return fields
}

type fieldScope struct {
	dot       string
	variables map[string]string
}

func (s fieldScope) fork() fieldScope {
	return fieldScope{dot: s.dot, variables: maps.Clone(s.variables)}
}

func fieldPath(t *testing.T, node parse.Node, scope fieldScope) string {
	t.Helper()

	switch node := node.(type) {
	case *parse.FieldNode:
		return joinField(scope.dot, node.Ident)
	case *parse.VariableNode:
		base, ok := scope.variables[node.Ident[0]]
		if !ok {
			t.Fatalf("unresolved template variable %s", node)
		}

		return joinField(base, node.Ident[1:])
	case *parse.DotNode:
		return scope.dot
	case *parse.ChainNode:
		return joinField(fieldPath(t, node.Node, scope), node.Field)
	case *parse.PipeNode:
		if len(node.Cmds) == 1 && len(node.Cmds[0].Args) == 1 {
			return fieldPath(t, node.Cmds[0].Args[0], scope)
		}
	}

	return ""
}

func joinField(base string, fields []string) string {
	if base == "" {
		return strings.Join(fields, ".")
	}

	if len(fields) == 0 {
		return base
	}

	return base + "." + strings.Join(fields, ".")
}

func walkFields(t *testing.T, node parse.Node, scope fieldScope, fields map[string]bool) {
	t.Helper()

	if node == nil || reflect.ValueOf(node).IsNil() {
		return
	}

	switch node := node.(type) {
	case *parse.ListNode:
		for _, child := range node.Nodes {
			walkFields(t, child, scope, fields)
		}
	case *parse.ActionNode:
		walkFields(t, node.Pipe, scope, fields)
	case *parse.PipeNode:
		walkPipe(t, node, scope, fields)
	case *parse.CommandNode:
		for _, argument := range node.Args {
			walkFields(t, argument, scope, fields)
		}
	default:
		walkControl(t, node, scope, fields)
	}
}

func walkFieldValue(t *testing.T, node parse.Node, scope fieldScope, fields map[string]bool) {
	t.Helper()

	switch node.(type) {
	case *parse.FieldNode, *parse.VariableNode, *parse.DotNode, *parse.ChainNode:
		path := strings.TrimSuffix(fieldPath(t, node, scope), "[]")
		if path != "" {
			fields[path] = true
		}
	case *parse.TextNode, *parse.StringNode, *parse.NumberNode, *parse.BoolNode, *parse.NilNode,
		*parse.IdentifierNode, *parse.CommentNode, *parse.BreakNode, *parse.ContinueNode:
	default:
		t.Fatalf("unsupported template node %T: %s", node, node)
	}
}

func walkBranch(t *testing.T, branch *parse.BranchNode, scope fieldScope, fields map[string]bool, changesDot bool) {
	t.Helper()

	inner := scope.fork()
	walkFields(t, branch.Pipe, inner, fields)
	otherwise := inner.fork()

	if changesDot {
		inner.dot = fieldPath(t, branch.Pipe.Cmds[0].Args[0], scope)
		if branch.NodeType == parse.NodeRange {
			inner.dot += "[]"
			for index, variable := range branch.Pipe.Decl {
				path := inner.dot
				if len(branch.Pipe.Decl) > 1 && index == 0 {
					path = strings.TrimSuffix(path, "[]")
				}

				inner.variables[variable.Ident[0]] = path
			}
		}
	}

	walkFields(t, branch.List, inner, fields)
	walkFields(t, branch.ElseList, otherwise, fields)
}

func TestPromptFieldScopes(t *testing.T) {
	t.Parallel()

	source := `{{.Subject | toYaml}}{{.Subject.FormatSteps}}
{{range $key, $document := .Docs}}{{$key}}{{$document.FilePath}}{{end}}
{{range $criterion := .Subject.AcceptanceCriteria}}
{{range $step := $criterion.Steps}}{{range .Given}}{{.Statement}}{{end}}{{end}}
{{else}}{{$.Subject.ID}}{{end}}
{{with $selected := .Subject.RegistryDocument}}{{.Scratch}}{{else}}{{$selected.Path}}{{end}}
{{if $subject := .Subject}}{{$subject.Title}}{{else}}{{$subject.ID}}{{end}}`
	actual := slices.Sorted(maps.Keys(templateFields(t, source)))

	expected := []string{
		"Docs", "Docs[].FilePath", "Subject", "Subject.AcceptanceCriteria",
		"Subject.AcceptanceCriteria[].Steps", "Subject.AcceptanceCriteria[].Steps[].Given",
		"Subject.AcceptanceCriteria[].Steps[].Given[].Statement", "Subject.FormatSteps",
		"Subject.ID", "Subject.RegistryDocument", "Subject.RegistryDocument.Path",
		"Subject.RegistryDocument.Scratch", "Subject.Title",
	}
	if !slices.Equal(actual, expected) {
		t.Fatalf("reads = %v, want %v", actual, expected)
	}
}

func walkPipe(t *testing.T, pipe *parse.PipeNode, scope fieldScope, fields map[string]bool) {
	t.Helper()

	for _, command := range pipe.Cmds {
		walkFields(t, command, scope, fields)
	}

	for _, variable := range pipe.Decl {
		scope.variables[variable.Ident[0]] = fieldPath(t, pipe.Cmds[0].Args[0], scope)
	}
}

func walkControl(t *testing.T, node parse.Node, scope fieldScope, fields map[string]bool) {
	t.Helper()

	switch node := node.(type) {
	case *parse.IfNode:
		walkBranch(t, &node.BranchNode, scope, fields, false)
	case *parse.RangeNode:
		walkBranch(t, &node.BranchNode, scope, fields, true)
	case *parse.WithNode:
		walkBranch(t, &node.BranchNode, scope, fields, true)
	default:
		walkFieldValue(t, node, scope, fields)
	}
}
