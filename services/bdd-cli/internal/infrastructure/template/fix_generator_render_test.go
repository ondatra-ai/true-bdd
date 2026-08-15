package template_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

// The template renders a story it is handed; these pin what it must SAY
// about that story, because the defect they cover was invisible in the
// data and only existed in the prompt.
type fixSubject struct {
	ID                 string
	Title              string
	AsA                string
	IWant              string
	SoThat             string
	AcceptanceCriteria []story.AcceptanceCriterion
}

type failedCheck struct {
	SectionPath  string
	Question     string
	ActualAnswer string
	FixPrompt    string
}

type fixData struct {
	DocPaths    map[string]string
	Subject     fixSubject
	FailedCheck failedCheck
	UserAnswers map[string]string
	ResultPath  string
}

func statement(text string) []story.StepStatement {
	return []story.StepStatement{{Statement: text}}
}

func render(t *testing.T) string {
	t.Helper()

	loader := template.NewTemplateLoader[fixData](
		"../../../../../templates/us-checklist.fix-generator.prompt.tpl")

	out, err := loader.LoadTemplate(fixData{
		Subject: fixSubject{
			ID: "96.5", Title: "Document Summary On Demand",
			AsA: "Claude User", IWant: "a summary", SoThat: "I can triage",
			AcceptanceCriteria: []story.AcceptanceCriterion{
				{
					ID:          "AC-1",
					Description: "Claude must display a summary of 150 words or fewer",
					Steps: []story.ScenarioStep{{
						Given: statement("a Google Doc with at least 500 words of body text is shared"),
						When:  statement("the Claude User asks for a summary"),
						Then:  statement("Claude displays a summary of 150 words or fewer"),
					}},
				},
				{ID: "AC-2", Description: "Claude must display an error when the doc is not shared"},
			},
		},
		FailedCheck: failedCheck{
			SectionPath: "acceptance_criteria", Question: "does every AC have steps?",
			ActualAnswer: "fail", FixPrompt: "add the missing steps",
		},
		ResultPath: "tmp/fix.md",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	return out
}

// The defect (#61): the prompt listed descriptions only, so the model
// could not tell an AC that was missing steps from one that already had
// them — and rewrote all of them, dropping AC-1's "at least 500 words"
// qualifier in the process.
func TestFixGeneratorPromptShowsExistingSteps(t *testing.T) {
	t.Parallel()

	out := render(t)

	if !strings.Contains(out, "at least 500 words of body text is shared") {
		t.Fatal("AC-1's existing steps are absent from the prompt: the model cannot preserve what it cannot see")
	}

	if !strings.Contains(out, "Existing steps: NONE") {
		t.Fatal("an AC with no steps is not marked as such, so the failing one is indistinguishable")
	}
}

// Naming the constraint is what makes the difference actionable: the
// model is told to reproduce untouched criteria verbatim.
func TestFixGeneratorPromptDemandsUntouchedCriteriaSurvive(t *testing.T) {
	t.Parallel()

	out := render(t)

	for _, phrase := range []string{
		"Change ONLY what the failed check requires",
		"byte for byte",
		"an omitted criterion is a deleted criterion",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("the prompt never says %q", phrase)
		}
	}
}
