package commands

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func commitConceptInterfaceFix(item *template.ScenarioCheckData) error {
	if !item.ConceptsDeclared {
		return nil
	}

	for _, document := range item.TermDocuments {
		if document.Key == docs.KeyArchitectureYAML {
			return commitInterfaceFix(document, item.InterfaceDocuments)
		}
	}

	return nil
}
