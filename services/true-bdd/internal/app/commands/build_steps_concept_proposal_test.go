package commands_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
)

// conceptReplies answers each concept turn by its schema, keeping every request.
type conceptReplies struct {
	stopAtBody  bool
	bodyReached bool
	proposals   []string
	requests    []string
	proposed    []string
}

func (reply *conceptReplies) ExecutePromptWithSystem(_ context.Context, role provider.Role, _ string, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, schema string, _ ...provider.TurnOptions,
) (string, error) {
	if reply.stopAtBody && role == provider.RoleFix {
		reply.bodyReached = true

		return "", errors.New("body writing reached")
	}

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

func conceptHost(t *testing.T, reply ports.AIPort) commands.BuildStepsDeps {
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

	err = disk.CopyTree("true-bdd", filepath.Join(repository, "true-bdd"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models := testModels(t)

	deps.Evaluator = validate.NewChecklistEvaluator(reply, cfg, models)
	deps.FixGenerator = validate.NewFixPromptGeneratorWithPaths(reply, cfg, models,
		filepath.Join(repository, "templates/build-steps.fix-generator.system.prompt.tpl"),
		filepath.Join(repository, "templates/build-steps.fix-generator.prompt.tpl"))
	deps.FixApplier = validate.NewFixApplier(reply, cfg, models)
	deps.Critic = validate.NewStepCritic(reply, models,
		filepath.Join(repository, "templates/build-steps.critique.system.prompt.tpl"),
		filepath.Join(repository, "templates/build-steps.critique.prompt.tpl"))
	deps.TmpDir = t.TempDir()
	deps.UserInputCollector = input.NewUserInputCollectorFrom(strings.NewReader(strings.Repeat("1\n", 12)))
	deps.TableRenderer = runner.NewTableRenderer()

	return deps
}

func TestPatternChecklistBoundsRefusedProposalsAfterWritingSkeletons(t *testing.T) {
	reply := &conceptReplies{proposals: []string{
		`{"patterns":[{"pattern":"^Press Halve once for one\\.$","captures":[],"serves":"Press Halve once."}]}`,
	}}
	deps := conceptHost(t, reply)
	deps.UserInputCollector = input.NewUserInputCollectorFrom(strings.NewReader("1\n1\n1\n"))
	deps.TableRenderer = runner.NewTableRenderer()

	err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if err == nil || !strings.Contains(err.Error(), "pattern-bindings") {
		t.Fatalf("want a refused proposal, got %v", err)
	}

	const attempts = 3
	if len(reply.proposed) != attempts {
		t.Fatalf("want %d proposals within max_apply_attempts, got %d", attempts, len(reply.proposed))
	}

	if !strings.Contains(reply.proposed[0], `"refused"`) ||
		!strings.Contains(reply.proposed[1], `no pattern binds when step \"Press Halve once for two.\"`) {
		t.Fatalf("resend lacks the engine's findings: %s", reply.proposed[1])
	}

	_, err = os.Stat("tests/calc/steps")
	if err != nil {
		t.Fatalf("pattern F did not write skeletons: %v", err)
	}
}

func TestBareConceptRunResumesPendingSkeletonsWithoutGrouping(t *testing.T) {
	reply := &conceptReplies{}
	deps := conceptHost(t, reply)

	_, err := stepskeleton.WriteConcept("tests/calc/steps", "when", "halve", `^Press Halve once for (\w+)\.$`)
	if err != nil {
		t.Fatal(err)
	}

	before, err := disk.Read("tests/calc/steps/register.go")
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

	if !regexp.MustCompile(`Check answered.*check=pattern-bindings.*answer=pass`).MatchString(output.String()) {
		t.Fatalf("resumed capturing registration failed the code Q: %s", output.String())
	}

	after, err := disk.Read("tests/calc/steps/register.go")
	if err != nil || string(after) != string(before) {
		t.Fatalf("bare run changed registrations: %s, %v", after, err)
	}
}

func TestBuildStepsReachesBodyWritingBeforeAnyTestOrMainExists(t *testing.T) {
	repository, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	reply := &conceptReplies{stopAtBody: true, proposals: []string{
		`{"patterns":[{"pattern":"^Press Halve once for (\\w+)\\.$",` +
			`"captures":[{"name":"number","means":"The number entered."}],"serves":"Press Halve once."}]}`,
	}}
	deps := conceptHost(t, reply)

	module := "module fixture/buildtests\n\ngo 1.25.0\n\n" +
		"require github.com/ondatra-ai/true-bdd/pkg v0.0.0\n" +
		"replace github.com/ondatra-ai/true-bdd/pkg => " + filepath.Join(repository, "pkg") + "\n"

	err = disk.Write("go.mod", []byte(module), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if !reply.bodyReached {
		t.Fatalf("body writing was not reached: %v", err)
	}

	entries, err := os.ReadDir("tests/calc")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if entry.Name() != "steps" {
			t.Errorf("build steps wrote %s", entry.Name())
		}
	}
}

func TestBuildStepsCompilesANestedStepsPackageBeforeTestsExist(t *testing.T) {
	repository, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	reply := &conceptReplies{stopAtBody: true, proposals: []string{
		`{"patterns":[{"pattern":"^Press Halve once for (\\w+)\\.$",` +
			`"captures":[{"name":"number","means":"The number entered."}],"serves":"Press Halve once."}]}`,
	}}
	deps := conceptHost(t, reply)
	module := "module fixture/tests\n\ngo 1.25.0\n\n" +
		"require github.com/ondatra-ai/true-bdd/pkg v0.0.0\n" +
		"replace github.com/ondatra-ai/true-bdd/pkg => " + filepath.Join(repository, "pkg") + "\n"

	err = disk.Write("tests/go.mod", []byte(module), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	architectureFile := "docs/architecture/architecture.yaml"

	source, err := disk.Read(architectureFile)
	if err != nil {
		t.Fatal(err)
	}

	updated := strings.Replace(string(source), `"testing": {`, `"testing": {"config":"tests/go.mod",`, 1)
	updated = strings.ReplaceAll(updated, "./tests/calc/...", "./calc/...")

	err = disk.Write(architectureFile, []byte(updated), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", architectureFile, true)
	if !reply.bodyReached {
		t.Fatalf("nested Steps package did not reach body writing: %v", err)
	}
}
