package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"gopkg.in/yaml.v3"
)

const finishedService = "true-bdd"

func TestFinishedSuiteVerificationFitsTheConfiguredPromptLimit(t *testing.T) {
	execution, arch, reply := finishedVerificationExecution(t)
	items, discovery, spec := finishedVerificationItems(t)
	prompt := finishedVerificationPrompt(t)
	largest, label := 0, ""

	for _, item := range items {
		size := measureFinishedVerification(t, execution, arch, item, discovery, spec, &prompt, reply)
		if size > largest {
			largest, label = size, item.Label()
		}
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	limit, err := cfg.PromptLimitBytes(150000)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("largest %s: %d bytes; %d Items; limit %d", label, largest, len(items), limit)

	if largest >= limit {
		t.Fatalf("%s: %d >= %d", label, largest, limit)
	}

	const expectedLargest = 70083
	if largest != expectedLargest || label != `then concept "file content"` {
		t.Fatalf("largest %s: %d", label, largest)
	}

	const expectedItems = 19
	if len(items) != expectedItems {
		t.Fatalf("finished suite Items=%d", len(items))
	}
}

func measureFinishedVerification(t *testing.T, execution *buildStepsExecution, arch *architecture.Architecture,
	item *BuildStepsItem, discovery *conceptDiscovery, spec conceptSpecification,
	prompt *checklistmodels.PromptWithContext, reply *helperCritiqueReply,
) int {
	t.Helper()

	err := describeConceptItem(item, arch, spec, discovery, execution.runtimeAPI)
	if err != nil {
		t.Fatal(err)
	}

	_, err = execution.critique(t.Context(), item, prompt)
	if err != nil {
		t.Fatal(err)
	}

	size := len(reply.system) + len(reply.user)
	t.Logf("%s: %d bytes", item.Label(), size)

	for _, leak := range []string{"\nfixture_inputs:\n", "\nregistry: docs/scenarios.yaml\n"} {
		if strings.Contains(reply.user, leak) {
			t.Fatalf("%s leaks %q", item.Label(), leak)
		}
	}

	if item.Concept.Name == "file content" {
		for _, want := range []string{"func assertEveryFileMatches", "func assertAtomicReplacement", "runtime_api:",
			"step_arguments:", "forms:", "examples:", "model-schemes:"} {
			if !strings.Contains(reply.user, want) {
				t.Errorf("largest Item lacks %q", want)
			}
		}
	}

	return size
}

func finishedVerificationExecution(
	t *testing.T,
) (*buildStepsExecution, *architecture.Architecture, *helperCritiqueReply) {
	t.Helper()

	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	reply := &helperCritiqueReply{answer: `{"answer":"pass","findings":[]}`}
	execution := helperCritiqueExecution(t, reply)
	t.Chdir(root)

	execution.architectureFile = repairArchitecture

	arch, err := architecture.Load(repairArchitecture)
	if err != nil {
		t.Fatal(err)
	}

	err = execution.prepareRuntimeAPI(t.Context(), arch)
	if err != nil {
		t.Fatal(err)
	}

	return execution, arch, reply
}

func finishedVerificationItems(t *testing.T) ([]*BuildStepsItem, *conceptDiscovery, conceptSpecification) {
	t.Helper()

	scenarios, err := registry.NewRegistryLoader().Load(conceptRegistry)
	if err != nil {
		t.Fatal(err)
	}

	source, err := product.Load("docs/product/product.yaml")
	if err != nil {
		t.Fatal(err)
	}

	definitions, mappings, err := stepSpecification("docs/product/product.yaml", repairArchitecture)
	if err != nil {
		t.Fatal(err)
	}

	bindings, err := architecture.LoadConceptBindings(repairArchitecture)
	if err != nil {
		t.Fatal(err)
	}

	discovery := &conceptDiscovery{scenarios: scenarios, registryFile: conceptRegistry}

	recorded, err := stepskeleton.Concepts("tests/true-bdd/steps")
	if err != nil {
		t.Fatal(err)
	}

	return groupFinishedDefinitions(recorded, discovery.suiteSteps("tests/true-bdd")), discovery,
		conceptSpecification{source.Concepts, definitions, mappings, bindings}
}

func groupFinishedDefinitions(recorded []stepskeleton.ConceptDefinition, steps []catalogue.Step) []*BuildStepsItem {
	items := make([]*BuildStepsItem, 0)
	byConcept := make(map[string]*BuildStepsItem)

	for _, definition := range recorded {
		key := catalogue.ConceptKey(definition.Block, definition.Concept)

		item := byConcept[key]
		if item == nil {
			item = &BuildStepsItem{Suite: "tests/true-bdd/steps", Service: finishedService, subjectID: key,
				Concept: &BuildStepsConcept{Block: definition.Block, Name: definition.Concept}}
			byConcept[key] = item
			items = append(items, item)
		}

		item.Concept.Definitions = append(item.Concept.Definitions, definition.Definition)
		for _, step := range steps {
			if step.Block == definition.Block && catalogue.PatternCaptures(definition.Pattern, step.Text) != nil {
				item.Concept.Steps = append(item.Concept.Steps, step.Text)
			}
		}
	}

	return items
}

func finishedVerificationPrompt(t *testing.T) checklistmodels.PromptWithContext {
	t.Helper()

	content, err := disk.Read("true-bdd/checklists/build-steps.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var checklist checklistmodels.Checklist

	err = yaml.Unmarshal(content, &checklist)
	if err != nil {
		t.Fatal(err)
	}

	for _, section := range checklist.Sections {
		if section.ID == stepVerificationID {
			return checklistmodels.PromptWithContext{Index: 2, SectionID: "build steps", CriterionID: stepVerificationID,
				Prompt: section.ValidationPrompts[0]}
		}
	}

	t.Fatal("no shipped verification item")

	return checklistmodels.PromptWithContext{}
}
