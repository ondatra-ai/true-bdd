package commands_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// conceptReplies answers each concept turn by its schema, keeping every request.
type conceptReplies struct {
	proposals []string
	requests  []string
	proposed  []string
}

func (reply *conceptReplies) ExecutePromptWithSystem(_ context.Context, _ provider.Role, _ string, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, schema string, _ ...provider.TurnOptions,
) (string, error) {
	reply.requests = append(reply.requests, schema)

	switch {
	case strings.Contains(schema, `"assignments"`):
		return `{"assignments":[` + groupingAnswer(strings.Count(user, `"key"`)) + `]}`, nil
	case strings.Contains(schema, `"patterns"`):
		reply.proposed = append(reply.proposed, user)
		answer := reply.proposals[min(len(reply.proposed), len(reply.proposals))-1]

		return answer, nil
	default:
		return `{"bindings":{"when:halve":{"unambiguous":true,"reason":"Press the named button once."}}}`, nil
	}
}

func groupingAnswer(texts int) string {
	answers := make([]string, 0, texts)
	for index := range texts {
		answers = append(answers, `{"index":`+string(rune('1'+index))+
			`,"concept":"halve","score":10,"runner_up":"double","runner_up_score":0}`)
	}

	return strings.Join(answers, ",")
}

func conceptHost(t *testing.T, reply *conceptReplies) commands.BuildStepsDeps {
	t.Helper()

	repository, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	deps := buildFixture(t)

	err = disk.CopyTree(".", filepath.Join(repository, "tests/true-bdd/fixtures/concept-grouping-batches/input"),
		disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.CopyTree("true-bdd/checklists", filepath.Join(repository, "true-bdd/checklists"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models, err := provider.NewRegistry(map[string]string{groupTestTier: "claude:test"}, map[provider.Role]string{
		provider.RolePrompt: groupTestTier, provider.RoleFix: groupTestTier, provider.RoleApply: groupTestTier,
	})
	if err != nil {
		t.Fatal(err)
	}

	deps.Evaluator = validate.NewChecklistEvaluator(reply, cfg, models)
	deps.FixGenerator = validate.NewFixPromptGenerator(reply, cfg, models)
	deps.FixApplier = validate.NewFixApplier(reply, cfg, models)
	deps.Critic = validate.NewStepCritic(reply, models,
		filepath.Join(repository, "templates/build-steps.critique.system.prompt.tpl"),
		filepath.Join(repository, "templates/build-steps.critique.prompt.tpl"))
	deps.TmpDir = t.TempDir()

	return deps
}

func TestRefusedPatternProposalGoesBackWithFindingsAndWritesNoSkeleton(t *testing.T) {
	reply := &conceptReplies{proposals: []string{
		`{"patterns":[{"pattern":"^Press Halve once for one\\.$","captures":[],"serves":"Press Halve once."}]}`,
	}}
	deps := conceptHost(t, reply)

	err := commands.RunBuildTests(t.Context(), commands.BuildTestsDeps{
		RegistryLoader: deps.RegistryLoader, DocResolver: deps.DocResolver,
	}, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if err != nil {
		t.Fatal(err)
	}

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if !errors.Is(err, commands.ErrPatternProposal) {
		t.Fatalf("want a refused proposal, got %v", err)
	}

	const attempts = 3
	if len(reply.proposed) != attempts {
		t.Fatalf("want %d proposals within max_apply_attempts, got %d", attempts, len(reply.proposed))
	}

	if strings.Contains(reply.proposed[0], `"refused"`) ||
		!strings.Contains(reply.proposed[1], `no pattern binds when step \"Press Halve once for two.\"`) {
		t.Fatalf("resend lacks the engine's findings: %s", reply.proposed[1])
	}

	_, err = os.Stat("tests/calc/steps")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused proposal wrote a skeleton: %v", err)
	}
}

func TestBareConceptRunResumesPendingSkeletonsWithoutGrouping(t *testing.T) {
	reply := &conceptReplies{}
	deps := conceptHost(t, reply)

	_, err := stepskeleton.WriteConcept("tests/calc/steps", "when", "halve", `^Press Halve once for (\w+)\.$`)
	if err != nil {
		t.Fatal(err)
	}

	var output strings.Builder

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
	if !errors.Is(err, commands.ErrMissingStepDefinitions) {
		t.Fatal(err)
	}

	if len(reply.requests) != 1 || !strings.Contains(reply.requests[0], `"bindings"`) {
		t.Fatalf("resume must rejudge the binding without grouping: %v", reply.requests)
	}

	if !strings.Contains(output.String(), `concept="when concept \"halve\"" steps=5 resumed=true`) {
		t.Fatalf("resumed Item not listed: %s", output.String())
	}
}
