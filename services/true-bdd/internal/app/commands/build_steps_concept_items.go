package commands

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// collect builds one Item per concept and suite from the grouped steps and
// from the pending skeletons an interrupted walk left, which it resumes.
func (d *conceptDiscovery) collect(suites []*template.ScenarioCheckData,
	groups map[string]map[catalogue.Step]string,
) error {
	d.steps = make(map[string][]catalogue.SuiteStep)

	for _, suite := range suites {
		dir := filepath.ToSlash(filepath.Join(suite.ID, "steps"))

		definitions, err := stepskeleton.Concepts(dir)
		if err != nil {
			return fmt.Errorf("read concept definitions: %w", err)
		}

		bound, err := stepcoverage.LoadSuite(dir)
		if err != nil {
			return err
		}

		items := make(map[string]*BuildStepsItem)
		order := make([]string, 0)

		for _, step := range d.suiteSteps(suite.ID) {
			entry := catalogue.SuiteStep{Block: step.Block, Text: step.Text}
			if concept, grouped := groups[suite.ID][step]; grouped {
				entry.Item = conceptKey(step.Block, concept)
				order = d.addConceptStep(items, order, suite, dir, entry, concept, definitions)
			} else if definition, resumed := pendingBinding(bound.Bind(step.Text), definitions); resumed {
				entry.Bound = definition.Pattern
				order = d.resumeConceptStep(items, order, suite, dir, entry, definition, definitions)
			} else if found := bound.Bind(step.Text); len(found) == 1 {
				entry.Bound = found[0].Pattern
			}

			d.steps[dir] = append(d.steps[dir], entry)
		}

		for _, key := range order {
			d.items = append(d.items, items[key])
		}
	}

	return nil
}

func (d *conceptDiscovery) addConceptStep(items map[string]*BuildStepsItem, order []string,
	suite *template.ScenarioCheckData, dir string, entry catalogue.SuiteStep, concept string,
	definitions []stepskeleton.ConceptDefinition,
) []string {
	item, known := items[entry.Item]
	if !known {
		item = newConceptItem(suite, dir, entry.Block, concept, definitions)
		items[entry.Item] = item
		order = append(order, entry.Item)
	}

	item.Concept.Steps = append(item.Concept.Steps, entry.Text)

	return order
}

func (d *conceptDiscovery) resumeConceptStep(items map[string]*BuildStepsItem, order []string,
	suite *template.ScenarioCheckData, dir string, entry catalogue.SuiteStep,
	definition stepskeleton.ConceptDefinition, definitions []stepskeleton.ConceptDefinition,
) []string {
	key := conceptKey(catalogue.Block(definition.Block), definition.Concept)

	item, known := items[key]
	if !known {
		item = newConceptItem(suite, dir, catalogue.Block(definition.Block), definition.Concept, definitions)
		items[key] = item
		order = append(order, key)
		suite.ConceptLinks = append(suite.ConceptLinks,
			template.ConceptLink{Concept: definition.Concept, Block: catalogue.Block(definition.Block)})
	}

	item.Concept.Resumed = true
	item.Concept.Steps = append(item.Concept.Steps, entry.Text)
	suite.CheckedSteps = append(suite.CheckedSteps, catalogue.Step{Block: entry.Block, Text: entry.Text})

	if !slices.Contains(item.Concept.Definitions, definition.Definition) {
		item.Concept.Definitions = append(item.Concept.Definitions, definition.Definition)
	}

	return order
}

func newConceptItem(suite *template.ScenarioCheckData, dir string, block catalogue.Block, concept string,
	definitions []stepskeleton.ConceptDefinition,
) *BuildStepsItem {
	item := &BuildStepsItem{Suite: dir, Service: suite.Service,
		Concept: &BuildStepsConcept{Block: block, Name: concept}}

	for _, definition := range definitions {
		if definition.Block == string(block) && definition.Concept == concept {
			item.Concept.Existing = append(item.Concept.Existing, definition.Pattern)
		}
	}

	return item
}

// pendingBinding is the pending concept skeleton a bound step's one definition is.
func pendingBinding(bound []catalogue.StepDefinition, definitions []stepskeleton.ConceptDefinition,
) (stepskeleton.ConceptDefinition, bool) {
	if len(bound) != 1 {
		return stepskeleton.ConceptDefinition{}, false
	}

	for _, definition := range definitions {
		if definition.Pattern == bound[0].Pattern && definition.Pending() {
			return definition, true
		}
	}

	return stepskeleton.ConceptDefinition{}, false
}

// suiteSteps are the deterministic steps of every Scenario of the suite,
// every block, each block and text once.
func (d *conceptDiscovery) suiteSteps(suite string) []catalogue.Step {
	steps := make([]catalogue.Step, 0)

	for _, scenario := range d.scenarios {
		if filepath.Dir(scenario.Path) != suite {
			continue
		}

		for _, step := range scenario.CheckedSteps() {
			if !step.ModelRun() && !slices.Contains(steps, step) {
				steps = append(steps, step)
			}
		}
	}

	return steps
}

func conceptKey(block catalogue.Block, concept string) string {
	return string(block) + ":" + concept
}
