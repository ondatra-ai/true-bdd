package commands

import (
	"crypto/sha256"
	"fmt"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// Expected hashes pin prompts without runtime API instructions; #303 names the checklist's pattern judgement.
func TestRuntimePromptInstructionsPreserveUndeclaredHostBytes(t *testing.T) {
	repairExecution(t, &proposalReplies{})

	item := &BuildStepsItem{Suite: repairSuite, Service: "calc", Context: "concept: halving\n",
		Concept: &BuildStepsConcept{Name: "halving", Means: "halve the number"}}
	checkRuntimePromptBytes(t, "fix-generator.system", validate.FixPromptData{Subject: item},
		"b91e0778dfb97c4c95e522c36e7099339de96f9ab8cb389ea3c36a415f36f11e")
	checkRuntimePromptBytes(t, "fix-generator", validate.FixPromptData{Subject: item},
		"629e60fc3da43e92aceb034d827fe93a024b47883e51b2ac626bb29a0d36f74f")
	checkRuntimePromptBytes(t, "fix-applier.system", validate.FixApplierData{Subject: item},
		"912e4d5b43c3ad811dc962c3c1406a6dafd624e0866db7f1e115355ee29cea5d")
	checkRuntimePromptBytes(t, "fix-applier", validate.FixApplierData{Subject: item},
		"2301b3442b5ba4703757c95960c28765275cca8ec839244420939b2fa4b0d422")
	checkRuntimePromptBytes(t, "critique.system", validate.StepCritiqueData{Context: item.Context},
		"c276012d02df2657eba73efcddfc29d1a70a65d8c129eebb76470de200ab4d69")
	checkRuntimePromptBytes(t, "critique", validate.StepCritiqueData{Context: item.Context},
		"c430f53174e8622477ee35baeaa5e6227730071628d459ea03e0f198cc0e8e36")
}

func TestBodyProposalDescribesOnlyTheSuppliedFindings(t *testing.T) {
	repairExecution(t, &proposalReplies{})

	const (
		missing = "This check found only that the bodies are not written"
		repair  = "This check's findings are listed under Validation Failure; this F fixes only those findings"
	)

	for _, sample := range []struct {
		name     string
		findings []string
		want     string
	}{
		{"missing bodies", []string{"step not written yet: first", "step not written yet: second"}, missing},
		{"pending body after apply", []string{"step not written yet"}, missing},
		{"compilation", []string{"Steps package does not compile: undefined: result"}, repair},
		{"skeleton integrity", []string{"step not written yet: first", "registration changed"}, repair},
		{"body critique", []string{"computes_assertion: computes the expected result"}, repair},
		{"no findings", nil, repair},
	} {
		t.Run(sample.name, func(t *testing.T) {
			item := &BuildStepsItem{Suite: repairSuite, Service: "calc",
				Concept: &BuildStepsConcept{Name: "halving", Means: "halve the number"}}
			data := validate.FixPromptData{Subject: item,
				FailedCheck: checklist.ValidationResult{Context: sample.findings}}

			out, err := template.NewTemplateLoader[validate.FixPromptData](
				"templates/build-steps.fix-generator.prompt.tpl").LoadTemplate(data)
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(out, sample.want) || strings.Contains(out, missing) && sample.want != missing {
				t.Fatalf("proposal misstates its findings: %s", out)
			}

			for _, finding := range sample.findings {
				if !strings.Contains(out, "- "+finding) {
					t.Errorf("Validation Failure omits %q", finding)
				}
			}
		})
	}
}

func checkRuntimePromptBytes[T any](t *testing.T, name string, data T, want string) {
	t.Helper()

	rendered, err := template.NewTemplateLoader[T]("templates/build-steps." + name + ".prompt.tpl").LoadTemplate(data)
	if err != nil {
		t.Fatal(err)
	}

	got := fmt.Sprintf("%x", sha256.Sum256([]byte(rendered)))
	if got != want {
		t.Errorf("%s prompt bytes: got %s, want %s", name, got, want)
	}
}

func TestBodyVerificationReceivesCompleteArgumentSectionWithoutTools(t *testing.T) {
	turn := &bodyEvidenceTurn{}
	execution := testkitExecution(t, turn)

	const section = "step_arguments:\n- {name: regex, read: Whole tail., applies: stdout, escaping: none}\n"

	body, err := disk.Read("docs/architecture/architecture.yaml")
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write("docs/architecture/architecture.yaml", append(body, []byte(section)...), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	execution.deps.Critic = validate.NewStepCritic(turn, testkitModels(t),
		"templates/build-steps.critique.system.prompt.tpl", "templates/build-steps.critique.prompt.tpl")
	item := &BuildStepsItem{Suite: repairSuite, Service: repairService,
		Concept: &BuildStepsConcept{Name: repairConcept}}

	_, _ = execution.critique(t.Context(), item, verificationTestPrompt())
	if !strings.Contains(turn.user, section) || len(turn.mode.DisallowedTools) == 0 {
		t.Fatalf("verification lost arguments or gained tools: %s, %+v", turn.user, turn.mode)
	}
}
