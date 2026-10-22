package checks

import (
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

// givenWhenThen passes a Scenario with at least one step of each kind:
// one missing has no precondition, no action, or nothing asserted.
func givenWhenThen() Check[*template.ScenarioCheckData] {
	return Check[*template.ScenarioCheckData]{
		Params: nil,
		Docs:   nil,
		Run: func(item *template.ScenarioCheckData, _ Input) (Verdict, error) {
			kinds := []struct {
				block catalogue.Block
				steps []string
			}{
				{catalogue.BlockGiven, item.Steps.Given},
				{catalogue.BlockWhen, item.Steps.When},
				{catalogue.BlockThen, item.Steps.Then},
			}

			context := make([]string, 0, len(kinds))

			for _, kind := range kinds {
				if len(kind.steps) == 0 {
					context = append(context, item.ID+": missing "+string(kind.block)+" step")
				}
			}

			if len(context) > 0 {
				return Verdict{Pass: false, Context: context}, nil
			}

			return Verdict{Pass: true, Context: []string{item.ID + ": ok"}}, nil
		},
	}
}
