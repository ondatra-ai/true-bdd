package verification_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/verification"
)

const (
    runtimeReady = "Ready"
    runtimeDone = "Done"
)

func TestRuntimeDocumentationKeepsCalledMeaningAndItsTypes(t *testing.T) {
	doc := `package runtime
FUNCTIONS
func Observe() *Result
    Observe returns the retained observation.
func Uncalled()
    Uncalled is unrelated.
TYPES
type Result struct {
    Value *Value
}
type Value struct { Text string }
func (r *Result) Unused()
    Unused is not called.
`

	selected := verification.RuntimeDocumentation(doc, `func selected() { runtime.Observe() } // Uncalled`)
	for _, want := range []string{"Observe returns", "type Result", "type Value"} {
		if !strings.Contains(selected, want) {
			t.Errorf("missing %q: %s", want, selected)
		}
	}

	for _, unwanted := range []string{"Uncalled", "Unused"} {
		if strings.Contains(selected, unwanted) {
			t.Errorf("uncalled declaration %q: %s", unwanted, selected)
		}
	}
}

func TestRuntimeDocumentationKeepsTypedIotaGroup(t *testing.T) {
	doc := `package runtime
CONSTANTS
const (
    Ready Mode = iota
    Done
)
    Ready and Done describe the lifecycle.
TYPES
type Mode int
`

	selected := verification.RuntimeDocumentation(doc, `func selected() { use(runtime.Ready) }`)
	for _, want := range []string{
        "Ready Mode = iota", runtimeDone, "Ready and Done describe the lifecycle.", "type Mode int",
    } {
		if !strings.Contains(selected, want) {
			t.Errorf("missing %q: %s", want, selected)
		}
	}
}

func TestRuntimeDocumentationKeepsEveryValueDeclarationForm(t *testing.T) {
	for _, sample := range []struct {
		name, group string
		members     []string
	}{
		{"untyped constant", "const (\n    Ready = 7\n)", []string{runtimeReady}},
		{"typed constant", "const (\n    Ready Mode = 7\n)", []string{runtimeReady}},
		{"untyped iota", "const (\n    Ready = iota\n    Done\n)", []string{runtimeReady, runtimeDone}},
		{"typed iota", "const (\n    Ready Mode = iota\n    Done\n)", []string{runtimeReady, runtimeDone}},
		{"implicit untyped repetition", "const (\n    Ready = 7\n    Done\n)", []string{runtimeDone}},
		{"implicit typed repetition", "const (\n    Ready Mode = 7\n    Done\n)", []string{runtimeDone}},
		{"multiple untyped constants", "const (\n    Ready, Done = 1, 2\n)", []string{runtimeReady, runtimeDone}},
		{"multiple typed constants", "const (\n    Ready, Done Mode = 1, 2\n)", []string{runtimeReady, runtimeDone}},
		{"multiple implicit constants", "const (\n    Ready, Done = 1, 2\n    Next, Last\n)", []string{"Next", "Last"}},
		{"untyped variable", "var (\n    Ready = makeMode(\"Ready)\")\n)", []string{runtimeReady}},
		{"typed initialized variable", "var (\n    Ready Mode = makeMode(\"Ready)\")\n)", []string{runtimeReady}},
		{"typed zero variable", "var (\n    Ready Mode\n)", []string{runtimeReady}},
		{"multiple untyped variables", "var (\n    Ready, Done = modes()\n)", []string{runtimeReady, runtimeDone}},
		{"multiple typed variables", "var (\n    Ready, Done Mode = 1, 2\n)", []string{runtimeReady, runtimeDone}},
		{"multiple zero variables", "var (\n    Ready, Done Mode\n)", []string{runtimeReady, runtimeDone}},
		{"unparenthesized constants", "const Ready, Done = 1, 2", []string{runtimeReady, runtimeDone}},
		{"unparenthesized typed constants", "const Ready, Done Mode = 1, 2", []string{runtimeReady, runtimeDone}},
		{"unparenthesized variables", "var Ready, Done = modes()", []string{runtimeReady, runtimeDone}},
		{"unparenthesized zero variables", "var Ready, Done Mode", []string{runtimeReady, runtimeDone}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			doc := "package runtime\n" + sample.group + "\n    Complete group meaning.\n" +
				"var (\n    Unused Mode\n)\n    Unrelated meaning.\ntype Mode int\n"
			for _, member := range sample.members {
				selected := verification.RuntimeDocumentation(doc, "use(runtime."+member+")")
				for _, want := range []string{sample.group, "Complete group meaning."} {
					if !strings.Contains(selected, want) {
						t.Errorf("reference %s lost %q: %s", member, want, selected)
					}
				}

				if strings.Contains(sample.group, " Mode") && !strings.Contains(selected, "type Mode int") {
					t.Errorf("reference %s lost its type: %s", member, selected)
				}

				if strings.Contains(selected, "Unused") {
					t.Errorf("reference %s selected an unrelated group: %s", member, selected)
				}
			}
		})
	}
}
