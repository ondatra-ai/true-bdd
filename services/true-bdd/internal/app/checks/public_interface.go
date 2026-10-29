package checks

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func publicInterface() Check[*template.ScenarioCheckData] {
	return Check[*template.ScenarioCheckData]{
		Docs: []string{docs.KeyArchitectureYAML},
		Run: func(item *template.ScenarioCheckData, input Input) (Verdict, error) {
			services, err := architecture.LoadInterfaces(input.Docs[docs.KeyArchitectureYAML], ".")
			if err != nil {
				return Verdict{}, fmt.Errorf("read Public interfaces: %w", err)
			}

			finding := "is not declared in the architecture"

			for _, service := range services {
				if service.Name == item.Service {
					finding = service.PublicInterface.Finding()

					break
				}
			}

			if finding != "" {
				return Verdict{Pass: false, Context: []string{item.ID + ": service " + item.Service + " " + finding}}, nil
			}

			context := item.ID + ": service " + item.Service + " declares a valid Public interface"

			return Verdict{Pass: true, Context: []string{context}}, nil
		},
	}
}
