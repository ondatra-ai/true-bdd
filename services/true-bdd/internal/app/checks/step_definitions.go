package checks

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// stepDefinitions passes a Scenario each step of which binds one step
// definition of its suite, recording a declared concept of the step's block.
func stepDefinitions() Check[*template.ScenarioCheckData] {
	return closureCheck([]string{docs.KeyProduct},
		func(item *template.ScenarioCheckData, _ Input, source product.Document) (Verdict, error) {
			return reviewConceptDefinitions(item, source, stepcoverage.StepsDir(item.Path))
		})
}
