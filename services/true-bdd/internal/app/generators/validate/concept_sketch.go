package validate

import (
	"encoding/json"
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

func (e *ChecklistEvaluator) conceptSketch(prompt checklist.PromptWithContext, data *ChecklistPromptData) error {
	if prompt.Prompt.With["concepts"] != "sketch" {
		return nil
	}

	path, err := e.docResolver.Resolve(docs.KeyProduct)
	if err != nil {
		return fmt.Errorf("resolve sketch product: %w", err)
	}

	source, err := product.Load(path)
	if err != nil {
		return fmt.Errorf("read sketch product: %w", err)
	}

	if !source.ConceptsDeclared {
		return nil
	}

	productText, err := e.conceptProduct()
	if err != nil {
		return err
	}

	architecturePath, err := e.docResolver.Resolve(docs.KeyArchitectureYAML)
	if err != nil {
		return fmt.Errorf("resolve sketch interface kinds: %w", err)
	}

	services, err := architecture.LoadInterfaces(architecturePath, ".")
	if err != nil {
		return fmt.Errorf("read sketch interface kinds: %w", err)
	}

	encoded, err := json.Marshal(serviceInterfaceKinds(services))
	if err != nil {
		return fmt.Errorf("encode interface kinds: %w", err)
	}

	data.Docs = map[string]*docs.ArchitectureDoc{}
	data.Question = "For every AC, draft a runnable test sketch against its product concepts' closed means " +
		"and the service's kind of Public interface. Use explicit inputs, actions and objective assertions. " +
		"A concept does not need an architecture binding yet. Never require, read or invent " +
		"architecture bindings or helpers. " +
		"Pass when at least 80% of ACs are convertible. Record each sketch and the totals in context.\n" +
		"Service interface kinds: " + string(encoded) + "\nProduct document:\n" + productText
	data.FixTemplate = "Offer the Product Owner a Story rewrite using the concepts' means " +
		"and the service interface kind. " +
		"Preserve unaffected fields and steps. Do not require architecture bindings; apply only at the menu."

	return nil
}

func serviceInterfaceKinds(services []architecture.Service) map[string]string {
	kinds := make(map[string]string)

	for _, service := range services {
		if service.PublicInterface != nil {
			kinds[service.Name] = service.PublicInterface.Kind
		}
	}

	return kinds
}

func (e *ChecklistEvaluator) renderChecklistPrompts(prompt checklist.PromptWithContext, data ChecklistPromptData,
) (string, string, error) {
	err := e.conceptSketch(prompt, &data)
	if err != nil {
		return "", "", err
	}

	return e.renderPrompts(data)
}
