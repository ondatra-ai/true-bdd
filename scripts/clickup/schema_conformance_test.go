package clickup_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// ticket-schema.yaml is human-facing and lives with the skill; Go now shadows
// it, so the two can drift silently. This is what stops them. The precedent
// for a test reading a repo file is gates/conformance_test.go.
const schemaPath = "../../.claude/skills/task-handle/ticket-schema.yaml"

type readySchema struct {
	Fields map[string]struct {
		ID string `yaml:"id"`
	} `yaml:"fields"`
	Body struct {
		RequiredHeadings []struct {
			Name string `yaml:"name"`
		} `yaml:"required_headings"`
	} `yaml:"body"`
}

func loadSchema(t *testing.T) readySchema {
	t.Helper()

	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("reading %s: %v", schemaPath, err)
	}

	var schema readySchema

	err = yaml.Unmarshal(raw, &schema)
	if err != nil {
		t.Fatalf("parsing %s: %v", schemaPath, err)
	}

	return schema
}

// A UUID that drifts writes the right value into the wrong field, and nothing
// goes red until a person reads the Ticket.
func TestSchemaFieldIDsMatchTheConstants(t *testing.T) {
	t.Parallel()

	schema := loadSchema(t)

	for name, fieldID := range clickup.FieldIDs() {
		declared, ok := schema.Fields[name]
		if !ok {
			t.Errorf("%s declares no field %q, which fields.go writes", schemaPath, name)

			continue
		}

		if declared.ID != fieldID {
			t.Errorf("field %q: %s says %s, fields.go says %s",
				name, schemaPath, declared.ID, fieldID)
		}
	}
}

// The headings are ticket.yaml's; ticket-schema.yaml must not carry a second
// list that says something else.
func TestSchemaHeadingsMatchTicketYAML(t *testing.T) {
	t.Parallel()

	schema := loadSchema(t)

	declared := make([]string, 0, len(schema.Body.RequiredHeadings))
	for _, heading := range schema.Body.RequiredHeadings {
		declared = append(declared, heading.Name)
	}

	headings := clickup.Headings()
	if len(declared) != len(headings) {
		t.Fatalf("%s lists %d headings, ticket.yaml declares %d: %v vs %v",
			schemaPath, len(declared), len(headings), declared, headings)
	}

	for index, name := range headings {
		if declared[index] != "### "+name {
			t.Errorf("heading %d: %s says %q, ticket.yaml says %q",
				index, schemaPath, declared[index], "### "+name)
		}
	}
}
