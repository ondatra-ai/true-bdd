package validate_test

import (
	"strings"
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

// The applier occasionally wraps its whole result in a markdown fence; see
// stripCodeFence. A real run's fence reached the YAML parser and failed an
// entire `us create --fix` run with "found character that cannot start any token".
func TestExtractFileContentStripsWholeBlockFence(t *testing.T) {
	t.Parallel()

	const path = "tmp/run/apply-result.yaml"

	response := "=== FILE_START: " + path + " ===\n" +
		"```yaml\nid: \"99.1\"\ntitle: \"Document Summary\"\n```\n" +
		"=== FILE_END: " + path + " ==="

	got := validate.ExtractFileContent(response, path)

	// The WHOLE block, not a prefix: a fence-stripper that ate the last
	// line would satisfy a prefix check and still corrupt the file.
	want := "id: \"99.1\"\ntitle: \"Document Summary\""
	if got != want {
		t.Fatalf("extraction = %q, want %q", got, want)
	}
}

// CommonMark fences are not always three backticks. A model that opens
// with ````yaml or ~~~ is doing nothing unusual, and an unmatched fence
// puts the YAML parse back where it started.
func TestExtractFileContentStripsTildeAndLongFences(t *testing.T) {
	t.Parallel()

	const path = "tmp/run/apply-result.yaml"

	for name, fence := range map[string][2]string{
		"tilde":       {"~~~yaml", "~~~"},
		"long ticks":  {"````yaml", "````"},
		"no language": {"```", "```"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			response := "=== FILE_START: " + path + " ===\n" +
				fence[0] + "\nid: \"99.1\"\n" + fence[1] + "\n" +
				"=== FILE_END: " + path + " ==="

			if got := validate.ExtractFileContent(response, path); got != "id: \"99.1\"" {
				t.Fatalf("extraction = %q, want the unfenced content", got)
			}
		})
	}
}

// A fence INSIDE the block belongs to the content: a fix prompt is
// markdown and legitimately shows example YAML.
func TestExtractFileContentKeepsInnerFences(t *testing.T) {
	t.Parallel()

	const path = "tmp/run/fix.md"

	body := "# Fix Prompt\n\nApply this:\n\n```yaml\n- ac_id: AC-1\n```\n\nThen re-check."
	response := "=== FILE_START: " + path + " ===\n" + body + "\n=== FILE_END: " + path + " ==="

	got := validate.ExtractFileContent(response, path)

	if !strings.Contains(got, "```yaml") {
		t.Fatalf("an inner fence was stripped, corrupting the content:\n%s", got)
	}
}
