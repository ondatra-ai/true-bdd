package validate_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func stepArgumentTemplateSubjects() map[string]any {
	return map[string]any{
		argumentStoryStem:    &template.StoryRefineData{},
		argumentApplyStem:    &template.ScenarioApplyData{},
		argumentScenarioStem: &template.ScenarioCheckData{},
		argumentStepsStem:    &commands.BuildStepsItem{Concept: &commands.BuildStepsConcept{}},
		argumentCodeStem:     &commands.BuildCodeItem{},
	}
}

func renderArgumentTemplate(t *testing.T, file string, data any) string {
	t.Helper()

	rendered, err := template.NewTemplateLoader[any](buildCodeTemplates + file + ".prompt.tpl").LoadTemplate(data)
	if err != nil {
		t.Fatal(err)
	}

	return rendered
}

func TestStepArgumentAbsentTemplateBytes(t *testing.T) {
	for stem, subject := range stepArgumentTemplateSubjects() {
		for _, role := range []string{argumentChecklistRole, argumentProposalRole, argumentApplicationRole} {
			if stem == argumentCodeStem && role == argumentChecklistRole {
				continue
			}

			name := stem + "." + role

			var data any

			switch role {
			case argumentChecklistRole:
				if stem == argumentStoryStem || stem == argumentApplyStem {
					name = stem
				}

				data = validate.ChecklistPromptData{Subject: subject, Question: argumentQuestion, ResultPath: argumentResultPath}
			case argumentProposalRole:
				data = validate.FixPromptData{Subject: subject, ResultPath: argumentResultPath}
			case argumentApplicationRole:
				data = validate.FixApplierData{Subject: subject, ResultPath: argumentResultPath}
			}

			got := renderArgumentTemplate(t, name, data)
			assertArgumentBaseline(t, name, got)
		}
	}

	got := renderArgumentTemplate(t, "build-steps.critique", validate.StepCritiqueData{})
	assertArgumentBaseline(t, "build-steps.critique", got)
}

func assertArgumentBaseline(t *testing.T, name, got string) {
	t.Helper()

	baselines := map[string]string{
		argumentStoryStem:                  "a967f0d3cfa533648ed756f3b6366d279b499e773508aaafdc1dc8869f5e1279",
		"us-checklist.fix-generator":       "939a2f9ca089f4ccd47995054097522d4882d4f2e97f8ad16db26d69e3047a77",
		"us-checklist.fix-applier":         "2e2619e5a15f38655e5040bba1177622ce807e1f8fbd78a0eabbe63a14459ca8",
		argumentApplyStem:                  "8e14431b7411fae24b71ff40f766a5661cafdee3e5ca1a44f59e48635a689905",
		"us.apply-checklist.fix-generator": "10ca310c1279150e372d4833b96f303ab59d73e0c4283e181bb87a1799bc289f",
		"us.apply-checklist.fix-applier":   "7114d604636391ad24dd0104e4960760d75e54bd305db9ce70edfa0fe0f94807",
		"scen-check.checklist":             "4f474933eaa2f28367680ce78f1399ba55ab665319fe6faeec317b586600ccd7",
		"scen-check.fix-generator":         "7aa29d17b5775db6b8f4765bbfb8b4aafb9b1b4d913e4a7d183f2bce4e469998",
		"scen-check.fix-applier":           "c9916966ea087ef8a65622b2a5f62abbfe082aa529910036b8cd6880ac7005e0",
		"build-steps.checklist":            "9ef356fabe84a0474f62cf2f23417c57282e44bbf37b1f45d0a9d11100cbc446",
		"build-steps.fix-generator":        "c163afd46d71ba16fa932e8470685f338088d9ee2a4ed8dfff17a3dd8a505112",
		"build-steps.fix-applier":          "5b68b846d82c7771a84901a4790da20cb846c0aff9a08b56f67165146057726c",
		"build-code.fix-generator":         "2e4f66d4aba9bef30e0eccd8de1186281a292f12c095a1746f1d02921a57399e",
		"build-code.fix-applier":           "2015a349a1f0a472c7f3433577404524d306cab7cf8801171c39ee16739ed690",
		"build-steps.critique":             "b133d3caef8b4b64c1035c0924fe890af18d5aaff2922b957bd9f59b59f496ee",
	}
	if hash := fmt.Sprintf("%x", sha256.Sum256([]byte(got))); hash != baselines[name] {
		t.Fatalf("%s absent-section bytes changed: %s", name, hash)
	}
}

const (
	argumentStoryStem         = "us-checklist"
	argumentApplyStem         = "us.apply-checklist"
	argumentScenarioStem      = "scen-check"
	argumentStepsStem         = "build-steps"
	argumentCodeStem          = "build-code"
	argumentProposalRole      = "fix-generator"
	argumentApplicationRole   = "fix-applier"
	argumentChecklistRole     = "checklist"
	argumentResultPath        = "tmp/result.yaml"
	argumentArchitecturePath  = "private-architecture.yaml"
	argumentArchitectureKey   = "architecture_yaml"
	argumentGroupJudgement    = "group"
	argumentProposeJudgement  = "propose"
	argumentBindingsJudgement = "bindings"
	argumentWordsJudgement    = "words"
)

const argumentQuestion = "Question"
