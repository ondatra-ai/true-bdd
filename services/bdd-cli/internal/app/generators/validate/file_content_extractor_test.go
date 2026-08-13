package validate_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
)

func TestExtractFileContent(t *testing.T) {
	path := "tmp/2026-03-08-18-10/apply-4.1-iter7-result.yaml"

	tests := []struct {
		name     string
		response string
		wantOK   bool
	}{
		{
			name:     "exact match",
			response: "=== FILE_START: " + path + " ===\n- id: AC-1\n=== FILE_END: " + path + " ===",
			wantOK:   true,
		},
		{
			name:     "extra quote after ===",
			response: `==="FILE_START: ` + path + ` ===` + "\n- id: AC-1\n=== FILE_END: " + path + " ===",
			wantOK:   true,
		},
		{
			name:     "quotes around FILE_START on both sides",
			response: `==="FILE_START: ` + path + `"===` + "\n- id: AC-1\n" + `==="FILE_END: ` + path + `"===`,
			wantOK:   true,
		},
		{
			name:     "no spaces around ===",
			response: "===FILE_START: " + path + "===\n- id: AC-1\n===FILE_END: " + path + "===",
			wantOK:   true,
		},
		{
			// crush routinely closes with a bare marker. The opening one
			// already named the file, so this block is complete.
			name:     "closing marker omits the path",
			response: "=== FILE_START: " + path + " ===\n- id: AC-1\n=== FILE_END ===",
			wantOK:   true,
		},
		{
			name:     "closing marker omits the path but keeps the colon",
			response: "=== FILE_START: " + path + " ===\n- id: AC-1\n=== FILE_END: ===",
			wantOK:   true,
		},
		{
			name:     "no match at all",
			response: "just some random text",
			wantOK:   false,
		},
		{
			// An opening marker with no close at all is a truncated
			// response, not a block — it must stay a miss.
			name:     "start marker with no close",
			response: "=== FILE_START: " + path + " ===\n- id: AC-1\n",
			wantOK:   false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := validate.ExtractFileContent(testCase.response, path)
			if testCase.wantOK && result == "" {
				t.Errorf("expected content but got empty string")
			}

			if !testCase.wantOK && result != "" {
				t.Errorf("expected empty string but got: %s", result)
			}

			if testCase.wantOK && result != "" && result != "- id: AC-1" {
				t.Errorf("expected '- id: AC-1' but got: %s", result)
			}
		})
	}
}
