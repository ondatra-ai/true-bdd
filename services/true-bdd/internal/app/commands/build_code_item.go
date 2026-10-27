package commands

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

// BuildCodeItem is one discovery's whole failing set, shrinking after each apply.
type BuildCodeItem struct {
	Tests     []*testrunner.FailingTest
	Documents map[string]string
	id        string
	title     string
}

// CodeFailureGroup retains every test identity beside the first whole output.
type CodeFailureGroup struct {
	IDs    []string
	Output string
}

// Groups compares normalized output without shortening the evidence shown to the writer.
func (item *BuildCodeItem) Groups() []CodeFailureGroup {
	groups := make([]CodeFailureGroup, 0)
	indexes := make(map[string]int)

	for _, test := range item.Tests {
		if test.LastRunPassed {
			continue
		}

		scenarioID := ""
		if test.Scenario != nil {
			scenarioID = test.Scenario.ID
		}

		key := normalizeRuntimeFailure(test.FailureOutput, test.TestName, scenarioID)

		index, exists := indexes[key]
		if !exists {
			index = len(groups)
			indexes[key] = index

			groups = append(groups, CodeFailureGroup{Output: test.FailureOutput})
		}

		groups[index].IDs = append(groups[index].IDs, test.ID)
	}

	return groups
}

// Scenarios lists each matched Scenario once, with its original steps.
func (item *BuildCodeItem) Scenarios() []*registry.RegistryScenario {
	scenarios := make([]*registry.RegistryScenario, 0)

	seen := make(map[string]bool)
	for _, test := range item.Tests {
		if test.LastRunPassed || test.Scenario == nil || seen[test.Scenario.ID] {
			continue
		}

		seen[test.Scenario.ID] = true
		scenarios = append(scenarios, test.Scenario)
	}

	return scenarios
}

// Specification unions the remaining members' phrases and words in discovery order.
func (item *BuildCodeItem) Specification() catalogue.Specification {
	spec := catalogue.Specification{}
	phrases, terms := make(map[string]bool), make(map[string]bool)

	for _, test := range item.Tests {
		if test.LastRunPassed {
			continue
		}

		spec.Declared = spec.Declared || test.Spec.Declared
		for _, phrase := range test.Spec.Phrases {
			if !phrases[phrase.Template] {
				spec.Phrases = append(spec.Phrases, phrase)
				phrases[phrase.Template] = true
			}
		}

		for _, term := range test.Spec.Terms {
			if !terms[term.Term] {
				spec.Terms = append(spec.Terms, term)
				terms[term.Term] = true
			}
		}
	}

	return spec
}

// BuildCodeChecks supplies the engine-owned verdict over the whole failing set.
func BuildCodeChecks() checks.Set[*BuildCodeItem] {
	return checks.Set[*BuildCodeItem]{"tests-pass": {
		Run: func(item *BuildCodeItem, _ checks.Input) (checks.Verdict, error) {
			groups := item.Groups()

			verdict := checks.Verdict{Pass: len(groups) == 0}
			for _, group := range groups {
				verdict.Context = append(verdict.Context, strings.Join(group.IDs, ", ")+": "+group.Output)
			}

			return verdict, nil
		}}}
}

func buildCodeSubject(item *BuildCodeItem) (string, string) { return item.id, item.title }
