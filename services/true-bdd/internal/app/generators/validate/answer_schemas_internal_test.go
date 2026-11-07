package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

func TestAnswerSchemasAcceptedByProviders(t *testing.T) {
	root := t.TempDir()
	installSchemaCapture(t, root)

	for name, schema := range engineAnswerSchemas(t) {
		for _, client := range []ai.Provider{ai.NewClaudeProvider(), ai.NewCodexProvider()} {
			t.Run(name+"/"+client.Name(), func(t *testing.T) {
				req := ai.Request{WorkDir: root, TmpDir: root, Label: name, ResultSchema: schema}

				_, err := client.Execute(t.Context(), req)
				if err != nil {
					t.Fatal(err)
				}

				rendered, err := disk.Read(filepath.Join(root, "captured.json"))
				if err != nil {
					t.Fatal(err)
				}

				var node map[string]any

				err = json.Unmarshal(rendered, &node)
				if err != nil {
					t.Fatal(err)
				}

				for _, keyword := range []string{"oneOf", "anyOf", "allOf"} {
					if _, present := node[keyword]; present {
						schemaRefusal(t, "$", keyword)
					}
				}

				walkAnswerSchema(t, node, "$", client.Name() == "codex")
				exportAnswerSchema(t, name, client.Name(), rendered)
			})
		}
	}
}

func engineAnswerSchemas(t *testing.T) map[string]string {
	t.Helper()

	concepts := []product.Concept{
		{Name: "halve", Block: catalogue.BlockWhen},
		{Name: "square", Block: catalogue.BlockWhen},
	}
	steps := []conceptStep{{Key: "1", Block: catalogue.BlockWhen}}

	link, err := linkSchema(steps, concepts[:1])
	if err != nil {
		t.Fatal(err)
	}

	emptyLink, err := linkSchema(steps, nil)
	if err != nil {
		t.Fatal(err)
	}

	groupSingle, err := groupSchema(steps, concepts[:1])
	if err != nil {
		t.Fatal(err)
	}

	group, err := groupSchema(steps, concepts)
	if err != nil {
		t.Fatal(err)
	}

	bindings, err := bindingSchema(map[string]linkedBinding{"when:halve@calc": {}})
	if err != nil {
		t.Fatal(err)
	}

	words, err := wordsSchema(map[string]product.Concept{"when:halve": concepts[0]})
	if err != nil {
		t.Fatal(err)
	}

	return map[string]string{
		"checklist": checklistResultSchema, "fix-proposal": fixResultSchema,
		"build-code-plan":  (&FixPromptGenerator{impossibleReport: true}).resultSchema(),
		"concept-proposal": proposalSchema, "step-critique": stepCritiqueSchema,
		"concept-link": link, "concept-link-empty": emptyLink,
		"concept-group": group, "concept-group-single": groupSingle,
		"concept-bindings": bindings, "concept-words": words,
	}
}

func installSchemaCapture(t *testing.T, root string) {
	t.Helper()

	scripts := map[string]string{
		"claude": `#!/bin/sh
while [ "$#" -gt 0 ]; do
 if [ "$1" = --json-schema ]; then printf '%s' "$2" > captured.json; fi
 shift
done
printf '%s\n' '{"type":"result","structured_output":{"answer":"pass"}}'
`,
		"codex": `#!/bin/sh
while [ "$#" -gt 0 ]; do
 case "$1" in
 --output-schema) /bin/cp "$2" captured.json ;;
 -o) printf '{}' > "$2" ;;
 esac
 shift
done
`,
	}
	for name, script := range scripts {
		path := filepath.Join(root, name)

		err := disk.Write(path, []byte(script), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}

		err = disk.Executable(path)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("PATH", root)
}

func walkAnswerSchema(t *testing.T, node map[string]any, path string, codex bool) {
	t.Helper()

	unsupported := []string{"minLength", "maxLength", "minimum", "maximum", "exclusiveMinimum",
		"exclusiveMaximum", "multipleOf", "minItems", "maxItems", "uniqueItems", "contains",
		"minContains", "maxContains", "$ref", "$recursiveRef", "$dynamicRef"}
	for _, keyword := range unsupported {
		if _, present := node[keyword]; present {
			schemaRefusal(t, path, keyword)
		}
	}

	checkSchemaObject(t, node, path, codex)

	for key, value := range node {
		walkSchemaChildren(t, key, value, path+"."+key, codex)
	}
}

func checkSchemaObject(t *testing.T, node map[string]any, path string, codex bool) {
	t.Helper()

	properties, hasProperties := node["properties"].(map[string]any)

	types, _ := node["type"].([]any)
	if hasProperties || node["type"] == "object" || slices.Contains(types, any("object")) {
		if node["additionalProperties"] != false {
			schemaRefusal(t, path, "additionalProperties must be false")
		}

		required, validRequired := node["required"].([]any)
		if !validRequired {
			schemaRefusal(t, path, "required must be an array, including for an empty object")
		}

		for name := range properties {
			if codex && !slices.Contains(required, any(name)) {
				schemaRefusal(t, path, "required omits "+name)
			}
		}
	}
}

func walkSchemaChildren(t *testing.T, key string, value any, path string, codex bool) {
	t.Helper()

	switch child := value.(type) {
	case map[string]any:
		if key == "properties" {
			for name, property := range child {
				schema, ok := property.(map[string]any)
				if !ok {
					t.Fatalf("%s.%s is not a schema object", path, name)
				}

				walkAnswerSchema(t, schema, path+"."+name, codex)
			}
		} else {
			walkAnswerSchema(t, child, path, codex)
		}
	case []any:
		for index, element := range child {
			if schema, ok := element.(map[string]any); ok {
				walkAnswerSchema(t, schema, fmt.Sprintf("%s[%d]", path, index), codex)
			}
		}
	}
}

func schemaRefusal(t *testing.T, path, detail string) {
	t.Helper()
	t.Errorf("%s: %s; guards #299 recording refusals: Claude 400 tools.5.custom.input_schema: "+
		"input_schema does not support oneOf, allOf, or anyOf at the top level; "+
		"Codex invalid_json_schema text.format.schema In context=('anyOf', '0'), "+
		"'additionalProperties' is required to be supplied and to be false", path, detail)
}

func exportAnswerSchema(t *testing.T, name, provider string, rendered []byte) {
	t.Helper()

	directory := os.Getenv("TRUEBDD_SCHEMA_EXPORT")
	if strings.TrimSpace(directory) == "" {
		return
	}

	var node any

	err := json.Unmarshal(rendered, &node)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(directory, name+"."+provider+".json")

	err = disk.Write(path, append(encoded, '\n'), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}
