package checks

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/stepcoverage"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

// stepDefinitions passes a Scenario every template of which binds to one
// step definition of its suite, each reached by that template alone,
// over the steps of every checked Scenario sharing the suite.
func stepDefinitions() Check[*template.ScenarioCheckData] {
	return closureCheck([]string{docs.KeyProduct},
		func(item *template.ScenarioCheckData, _ Input, source product.Catalogue) (Verdict, error) {
			dir := stepcoverage.StepsDir(item.Path)

			suite, err := stepcoverage.LoadSuite(dir)
			if err != nil {
				return Verdict{}, fmt.Errorf("read the step definitions in %s: %w", dir, err)
			}

			return reviewDefinitions(item, source, suite.Bind, dir), nil
		})
}

// reviewDefinitions holds one Item's templates to its suite's step
// definitions. A step bound to nothing yet is a gap `build tests`
// authors, not a failure.
func reviewDefinitions(
	item *template.ScenarioCheckData,
	source product.Catalogue,
	bind catalogue.Binder,
	dir string,
) Verdict {
	compiled, _ := catalogue.Compile(source.Entries, source.Roles)

	review := compiled.Correspond(item.Phrases, suiteSteps(item), bind, dir)
	if len(review.Findings) > 0 {
		return Verdict{Pass: false, Context: prefixed(item.ID, review.Findings)}
	}

	line := item.ID + ": each template binds to one step definition in " + dir

	switch len(review.Unbound) {
	case 0:
	case 1:
		line += "; 1 step unbound yet"
	default:
		line += fmt.Sprintf("; %d steps unbound yet", len(review.Unbound))
	}

	return Verdict{Pass: true, Context: append([]string{line}, prefixed(item.ID, review.Unbound)...)}
}

// suiteSteps lists the steps of every Scenario a Story claims whose test
// file shares this Item's suite, the Item's own among them.
func suiteSteps(item *template.ScenarioCheckData) []catalogue.Step {
	dir := stepcoverage.StepsDir(item.Path)
	steps := make([]catalogue.Step, 0)

	for _, scenario := range item.Registry {
		if !claimed(scenario) || stepcoverage.StepsDir(scenario.Path) != dir {
			continue
		}

		steps = append(steps, scenario.Phrases...)
	}

	return steps
}
