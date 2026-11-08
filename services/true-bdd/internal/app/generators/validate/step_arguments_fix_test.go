package validate_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func TestFixTurnsReceiveStepArguments(t *testing.T) {
	for stem := range stepArgumentTemplateSubjects() {
		for _, role := range []string{argumentProposalRole, argumentApplicationRole} {
			t.Run(stem+"/"+role, func(t *testing.T) {
				_, reply, repository := conceptRequestEvaluator(t)
				declareArgumentSection(t)

				subject, reference := argumentFixSubject(stem, role)
				if reference == "tmp/working-architecture.yaml" {
					body, err := disk.Read(argumentArchitecturePath)
					if err != nil {
						t.Fatal(err)
					}

					err = disk.Write(reference, body, disk.Shared)
					if err != nil {
						t.Fatal(err)
					}
				}

				invokeArgumentFix(t, reply, repository, stem, role, subject)
				requireArgumentEvidence(t, reply.user, reference)
			})
		}
	}
}

func argumentFixSubject(stem, role string) (any, string) {
	subject := stepArgumentTemplateSubjects()[stem]
	reference := ""

	switch item := subject.(type) {
	case *commands.BuildCodeItem:
		reference = argumentArchitecturePath
		item.Documents = map[string]string{argumentArchitectureKey: reference}
	case *template.ScenarioCheckData:
		if role == argumentApplicationRole {
			reference = "tmp/working-architecture.yaml"
			item.TermDocuments = []template.FixDocument{
				{Key: argumentArchitectureKey, Path: argumentArchitecturePath, Scratch: reference}}
		}
	case *template.ScenarioApplyData:
		if role == argumentProposalRole {
			reference = argumentArchitecturePath
		}
	}

	return subject, reference
}

func invokeArgumentFix(t *testing.T, reply *linkReply, repository, stem, role string, subject any,
	architecturePath ...string,
) {
	t.Helper()

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	system := filepath.Join(repository, "templates", stem+"."+role+".system.prompt.tpl")

	user := filepath.Join(repository, "templates", stem+"."+role+".prompt.tpl")
	if role == argumentProposalRole {
		generator := validate.NewFixPromptGeneratorWithPaths(reply, cfg, argumentModels(t), system, user)
		if len(architecturePath) > 0 {
			generator.UseStepArgumentArchitecture(architecturePath[0])
		}

		failed := checklist.ValidationResult{PromptIndex: 1}
		if stem == argumentApplyStem {
			failed.Docs = []string{argumentArchitectureKey}
		}

		_, _ = generator.Generate(t.Context(), validate.GenerateParams{Subject: subject,
			SubjectID: "argument-item", FailedCheck: failed, TmpDir: t.TempDir(), Iteration: 1})

		return
	}

	applier := validate.NewFixApplierWithPaths(reply, cfg, argumentModels(t), system, user)
	if len(architecturePath) > 0 {
		applier.UseStepArgumentArchitecture(architecturePath[0])
	}

	_, _ = applier.Apply(t.Context(), validate.ApplyParams{Subject: subject,
		SubjectID: "argument-item", TmpDir: t.TempDir()})
}

func requireArgumentEvidence(t *testing.T, request, reference string) {
	t.Helper()

	if reference == "" {
		if !strings.Contains(request, argumentSection) {
			t.Fatalf("lost complete argument section: %s", request)
		}

		return
	}

	if !strings.Contains(request, "Read(`"+reference+"`) — the complete top-level `step_arguments:`") ||
		strings.Contains(request, argumentSection) {
		t.Fatalf("lost section reference: %s", request)
	}
}

func TestFixTurnsReadTheSelectedArchitecture(t *testing.T) {
	for _, stem := range []string{argumentStepsStem, argumentCodeStem} {
		for _, role := range []string{argumentProposalRole, argumentApplicationRole} {
			t.Run(stem+"/"+role, func(t *testing.T) {
				_, reply, repository := conceptRequestEvaluator(t)

				body, err := disk.Read(argumentArchitecturePath)
				if err != nil {
					t.Fatal(err)
				}

				const selected = "selected-architecture.yaml"

				err = disk.Write(selected, append(body, []byte(argumentSection)...), disk.Shared)
				if err != nil {
					t.Fatal(err)
				}

				subject, _ := argumentFixSubject(stem, role)
				invokeArgumentFix(t, reply, repository, stem, role, subject, selected)

				reference := ""
				if stem == argumentCodeStem {
					reference = selected
				}

				requireArgumentEvidence(t, reply.user, reference)
			})
		}
	}
}
