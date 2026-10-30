package commands

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

const (
	repairSuite        = "tests/calc/steps"
	repairService      = "calc"
	repairConcept      = "halving"
	repairTier         = "high"
	repairArchitecture = "docs/architecture/architecture.yaml"
	halveSix           = "the Owner halves 6"
	wrongCapture       = `^the Owner halves (-?\d)\d*$`
	resultPattern      = `^the result is (-?\d+)$`
)

// proposalReplies answers every proposal turn with one fixed proposal, keeping each request.
type proposalReplies struct {
	answer   string
	requests []string
}

func (reply *proposalReplies) ExecutePromptWithSystem(_ context.Context, _ provider.Role, _ string, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, _ string, _ ...provider.TurnOptions,
) (string, error) {
	reply.requests = append(reply.requests, user)

	return reply.answer, nil
}

// A critique finding against a pattern sends the Item back to the proposal
// with that finding; the engine checks the new pattern, rewrites the
// skeleton and leaves the new body to be written.
func TestCritiquePatternFindingRepairsTheItemThroughANewProposal(t *testing.T) {
	reply := &proposalReplies{answer: `{"patterns":[{"pattern":"^the Owner halves (-?\\d+)$",` +
		`"captures":[{"name":"number","means":"The whole number halved."}],"serves":"Run halve."}]}`}
	execution := repairExecution(t, reply)

	wrong, err := stepskeleton.WriteConcept(repairSuite, "when", repairConcept, wrongCapture)
	if err != nil {
		t.Fatal(err)
	}

	item := &BuildStepsItem{Suite: repairSuite, Service: repairService, subjectID: "when-halving",
		Concept: &BuildStepsConcept{Block: catalogue.BlockWhen, Name: repairConcept, Resumed: true,
			Steps: []string{halveSix, "the Owner halves -7"}, Existing: []string{wrongCapture},
			Definitions: []stepskeleton.Definition{wrong}}}
	finding := validate.CritiqueFinding{Definition: wrong.File + "::" + wrong.Function,
		Category: "pattern_captures", Finding: "captures only the first digit of the number"}

	err = execution.applyCritique(t.Context(), item, []validate.CritiqueFinding{finding})
	if err != nil {
		t.Fatal(err)
	}

	if len(reply.requests) != 1 || !strings.Contains(reply.requests[0], `"refused"`) ||
		!strings.Contains(reply.requests[0], "captures only the first digit") ||
		!strings.Contains(reply.requests[0], `"existing_definitions": []`) {
		t.Fatalf("the proposal lacks the refused pattern and the critique's finding: %v", reply.requests)
	}

	requireRewrittenSkeleton(t, item, wrong)

	if !slices.Contains(item.Findings, finding.String()) ||
		!slices.ContainsFunc(item.Findings, func(line string) bool { return strings.Contains(line, "not written yet") }) {
		t.Fatalf("the new body is not sent to the writer with the finding: %v", item.Findings)
	}
}

func requireRewrittenSkeleton(t *testing.T, item *BuildStepsItem, wrong stepskeleton.Definition) {
	t.Helper()

	_, err := disk.Read(wrong.File)
	if err == nil {
		t.Fatal("the refused pattern's skeleton remains")
	}

	suite, err := stepskeleton.Concepts(repairSuite)
	if err != nil || len(suite) != 1 || suite[0].Pattern != `^the Owner halves (-?\d+)$` {
		t.Fatalf("skeletons after the repair: %+v, %v", suite, err)
	}

	if len(item.Concept.Definitions) != 1 || item.Concept.Definitions[0].Function != suite[0].Function ||
		!strings.Contains(item.Context, `pattern: ^the Owner halves (-?\d+)$`) {
		t.Fatalf("the Item does not hold the new definition: %+v\n%s", item.Concept.Definitions, item.Context)
	}
}

func repairExecution(t *testing.T, reply *proposalReplies) *buildStepsExecution {
	t.Helper()

	repository, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	t.Chdir(t.TempDir())

	for target, source := range map[string]string{
		".":         "tests/true-bdd/fixtures/concept-steps-negative/input",
		"true-bdd":  "true-bdd",
		"templates": "templates",
	} {
		err = disk.CopyTree(target, filepath.Join(repository, source), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = disk.Write("go.mod", []byte("module fixture/repair\n\ngo 1.25.0\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models, err := provider.NewRegistry(map[string]string{repairTier: "claude:test"}, map[provider.Role]string{
		provider.RolePrompt: repairTier, provider.RoleFix: repairTier, provider.RoleApply: repairTier,
	})
	if err != nil {
		t.Fatal(err)
	}

	deps := BuildStepsDeps{RegistryLoader: registry.NewRegistryLoader(), DocResolver: docs.NewResolver(cfg),
		ChecklistLoader: checklist.NewChecklistLoader(cfg), Evaluator: validate.NewChecklistEvaluator(reply, cfg, models),
		TmpDir: t.TempDir()}

	return repairRun(t, deps)
}

func repairRun(t *testing.T, deps BuildStepsDeps) *buildStepsExecution {
	t.Helper()

	scenarios, err := deps.RegistryLoader.Load("docs/scenarios.yaml")
	if err != nil {
		t.Fatal(err)
	}

	arch, err := architecture.Load(repairArchitecture)
	if err != nil {
		t.Fatal(err)
	}

	execution := &buildStepsExecution{deps: deps, architectureFile: repairArchitecture,
		compiler: testrunner.NewGoTestRunner(testrunner.NewArtifacts(deps.TmpDir)),
		command: testrunner.Config{Framework: testrunner.FrameworkGoTest,
			Command: "go test -json -count=1 ./tests/calc/steps/"}}

	discovery := &conceptDiscovery{scenarios: scenarios, steps: map[string][]catalogue.SuiteStep{repairSuite: {
		{Block: catalogue.BlockWhen, Text: halveSix, Bound: wrongCapture},
		{Block: catalogue.BlockThen, Text: "the result is 3", Bound: resultPattern},
		{Block: catalogue.BlockWhen, Text: "the Owner halves -7", Bound: wrongCapture},
		{Block: catalogue.BlockThen, Text: "the result is -3", Bound: resultPattern},
	}}}

	err = execution.prepareConceptRun(discovery, arch)
	if err != nil {
		t.Fatal(err)
	}

	return execution
}

// A critique finding against a body sends only the body back: no proposal
// turn, and the Item keeps its skeleton.
func TestCritiqueBodyFindingSendsOnlyTheBodyBack(t *testing.T) {
	reply := &proposalReplies{}
	execution := repairExecution(t, reply)

	written, err := stepskeleton.WriteConcept(repairSuite, "when", repairConcept, `^the Owner halves (-?\d+)$`)
	if err != nil {
		t.Fatal(err)
	}

	item := &BuildStepsItem{Suite: repairSuite, Service: repairService, Concept: &BuildStepsConcept{
		Block: catalogue.BlockWhen, Name: repairConcept, Definitions: []stepskeleton.Definition{written},
	}}
	finding := validate.CritiqueFinding{Definition: written.Function, Category: "concept_binding",
		Finding: "requires CALC_LICENCE, which neither the binding nor a fixture input states"}

	err = execution.applyCritique(t.Context(), item, []validate.CritiqueFinding{finding})
	if err != nil {
		t.Fatal(err)
	}

	if len(reply.requests) != 0 || len(item.Findings) != 1 || item.Findings[0] != finding.String() ||
		len(item.Concept.Definitions) != 1 || written.Intact(repairSuite) != nil {
		t.Fatalf("a body finding changed more than the body's findings: %v %v", reply.requests, item.Findings)
	}
}
