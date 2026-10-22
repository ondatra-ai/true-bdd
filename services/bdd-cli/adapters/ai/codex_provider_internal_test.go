package ai

import (
	"encoding/json"
	"slices"
	"testing"
)

// codex answers HTTP 400 `invalid_json_schema` to a schema whose `required`
// leaves a property out (codex-cli 0.162.1, #165); claude takes it as is.
func TestCodexStrictSchemaRequiresEveryProperty(t *testing.T) {
	t.Parallel()

	const schema = `{
	  "type": "object",
	  "properties": {
	    "answer": {"type": "string"},
	    "fix_prompt": {"type": "string"},
	    "questions": {
	      "type": "array",
	      "items": {
	        "type": "object",
	        "properties": {"id": {"type": "string"}, "options": {"type": "array"}},
	        "required": ["id"]
	      }
	    }
	  },
	  "required": ["answer"],
	  "additionalProperties": false
	}`

	strict, err := codexStrictSchema(schema)
	if err != nil {
		t.Fatalf("codexStrictSchema: %v", err)
	}

	var decoded struct {
		Required             []string `json:"required"`
		AdditionalProperties bool     `json:"additionalProperties"` //nolint:tagliatelle // JSON Schema's own key
		Properties           struct {
			Questions struct {
				Items struct {
					Required []string `json:"required"`
				} `json:"items"`
			} `json:"questions"`
		} `json:"properties"`
	}

	err = json.Unmarshal([]byte(strict), &decoded)
	if err != nil {
		t.Fatalf("strict schema is not JSON: %v\n%s", err, strict)
	}

	if want := []string{"answer", "fix_prompt", "questions"}; !slices.Equal(decoded.Required, want) {
		t.Errorf("top-level required = %v, want %v", decoded.Required, want)
	}

	if want := []string{"id", "options"}; !slices.Equal(decoded.Properties.Questions.Items.Required, want) {
		t.Errorf("nested required = %v, want %v", decoded.Properties.Questions.Items.Required, want)
	}

	if decoded.AdditionalProperties {
		t.Error("additionalProperties: false was lost")
	}
}

func TestCodexStrictSchemaRefusesANonJSONSchema(t *testing.T) {
	t.Parallel()

	_, err := codexStrictSchema("not json")
	if err == nil {
		t.Fatal("codexStrictSchema accepted a schema that is not JSON")
	}
}
