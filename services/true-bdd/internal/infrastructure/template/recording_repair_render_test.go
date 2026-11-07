package template_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

const (
	termMappingsCheck    = "term-mappings"
	stepDefinitionsCheck = "step-definitions"
	publicInterfaceCheck = "public-interface"
	declaredTermsCheck   = "declared-terms"
	wordsJudgement       = "words"
)

func repairPrompt(t *testing.T, command, role, check, judgement string) string {
	t.Helper()

	subject := &template.ScenarioCheckData{ID: "calc-001", Service: "calc"}
	if check == termMappingsCheck {
		subject.TermDocuments = []template.FixDocument{{Key: "architecture_yaml"}}
	}

	data := map[string]any{
		"Subject":   subject,
		"SubjectID": "calc-001", "Judgement": judgement, "Check": check,
		"FailedCheck": map[string]any{"Check": check, "Judgement": judgement},
		"UserAnswers": map[string]string{"q1": "A reply is the service's response."},
		"Structured":  true, "ResultPath": "tmp/fix.md",
	}
	loader := template.NewTemplateLoader[map[string]any]("../../../../../templates/" + command + "." + role + ".tpl")

	result, err := loader.LoadTemplate(data)
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func TestTermApplierHasAlreadyReceivedMenuApproval(t *testing.T) {
	t.Parallel()

	for _, check := range []string{
		declaredTermsCheck, termMappingsCheck, stepDefinitionsCheck, publicInterfaceCheck, "",
	} {
		out := repairPrompt(t, "scen-check", "fix-applier.system.prompt", check, "")

		want := check != publicInterfaceCheck && check != ""
		for _, phrase := range []string{
			"The user has already chosen Apply at the engine menu, authorizing this turn.",
			"Menu reminders inside the approved fix are satisfied; apply it without asking again.",
		} {
			if strings.Contains(out, phrase) != want {
				t.Errorf("check %q: approval instruction %q present=%v, want %v", check, phrase, !want, want)
			}
		}
	}
}

func TestFixProposalsUseSettledAnswersAndLeaveApprovalToEngine(t *testing.T) {
	t.Parallel()

	for _, check := range []string{
		declaredTermsCheck, termMappingsCheck, "concept-bindings", stepDefinitionsCheck, wordsJudgement, publicInterfaceCheck,
	} {
		judgement := ""
		if check == wordsJudgement {
			judgement = check
		}

		user := repairPrompt(t, "scen-check", "fix-generator.prompt", check, judgement)
		system := repairPrompt(t, "scen-check", "fix-generator.system.prompt", check, judgement)

		for _, phrase := range []string{"Owner's settled answers to propose from:",
			"Propose from these answers; ask only for meaning or operations they and the Specification do not give.",
			"A reply is the service's response."} {
			if !strings.Contains(user, phrase) {
				t.Errorf("check %q: user prompt lacks %q", check, phrase)
			}
		}

		for _, phrase := range []string{
			"The engine shows the fix prompt to its owner at the menu and applies it only after approval.",
			"The fix prompt is the edit to apply and asks for no approval."} {
			if strings.Contains(system, phrase) != (check != publicInterfaceCheck) {
				t.Errorf("check %q: misplaced system instruction %q", check, phrase)
			}
		}
	}
}

func TestGlossaryFormAppearsOnlyInGlossaryProposals(t *testing.T) {
	t.Parallel()

	const rule = "Each glossary entry's means must be one paragraph of at most three sentences, each at most 25 words."

	for _, command := range []string{"scen-check", "us-checklist"} {
		for _, judgement := range []string{wordsJudgement, "link", "bindings"} {
			out := repairPrompt(t, command, "fix-generator.prompt", "", judgement)
			if strings.Contains(out, rule) != (judgement == wordsJudgement) {
				t.Errorf("%s judgement %q: glossary form misplaced", command, judgement)
			}
		}
	}

	for _, check := range []string{
		declaredTermsCheck, termMappingsCheck, "concept-bindings", stepDefinitionsCheck, publicInterfaceCheck, "",
	} {
		out := repairPrompt(t, "scen-check", "fix-generator.system.prompt", check, "")
		if strings.Contains(out, rule) != (check == declaredTermsCheck) {
			t.Errorf("check %q: glossary form misplaced", check)
		}
	}
}

func TestJudgementFixProposalsHaveThreeOutcomes(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"scen-check", "us-checklist"} {
		user := repairPrompt(t, command, "fix-generator.prompt", "", wordsJudgement)
		system := repairPrompt(t, command, "fix-generator.system.prompt", "", wordsJudgement)

		for _, phrase := range []string{
			"Owner's settled answers to propose from:",
			"Propose from these answers; ask only for meaning or operations they and the Specification do not give.",
			"`cannot_fix`", "{reason, correction}", "Exactly one of the three carries content.",
		} {
			if !strings.Contains(user, phrase) {
				t.Errorf("%s judgement lacks %q", command, phrase)
			}
		}

		for _, phrase := range []string{
			"The engine shows the fix prompt to its owner at the menu and applies it only after approval.",
			"The fix prompt is the edit to apply and asks for no approval.",
		} {
			if !strings.Contains(system, phrase) {
				t.Errorf("%s judgement lacks %q", command, phrase)
			}
		}

		if strings.Contains(user, "Exactly one carries content:") || strings.Contains(user, "User answers:") {
			t.Errorf("%s judgement retains the obsolete contract", command)
		}
	}
}
