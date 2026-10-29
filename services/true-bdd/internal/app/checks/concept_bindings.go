package checks

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func reviewConceptBindings(item *template.ScenarioCheckData, input Input, concepts []product.Concept) (Verdict, error) {
	path := input.Docs[docs.KeyArchitectureYAML]

	bindings, err := architecture.LoadConceptBindings(path)
	if err != nil {
		return Verdict{}, fmt.Errorf("read Concept bindings: %w", err)
	}

	services, err := architecture.LoadInterfaces(path, ".")
	if err != nil {
		return Verdict{}, fmt.Errorf("read Public interfaces: %w", err)
	}

	findings := conceptBindingFindings(concepts, bindings, services)
	if len(findings) != 0 {
		return Verdict{Pass: false, Context: prefixed(item.ID, findings)}, nil
	}

	context := item.ID + ": every concept binding names a declared concept and valid Public interface references"

	return Verdict{Pass: true, Context: []string{context}}, nil
}

func conceptBindingFindings(concepts []product.Concept, bindings []architecture.ConceptBinding,
	services []architecture.Service,
) []string {
	findings := make([]string, 0)
	seen := make(map[[3]string]bool)

	for _, binding := range bindings {
		key := [3]string{binding.Concept, string(binding.Block), binding.Service}
		if seen[key] {
			continue
		}

		seen[key] = true

		subject := fmt.Sprintf("%s concept '%s' for service %s", binding.Block, binding.Concept, binding.Service)
		if !declaredConcept(concepts, binding) {
			findings = append(findings, subject+" names no declared concept")
		}

		entries := conceptBindingEntries(bindings, binding)

		declaration := bindingInterface(services, binding.Service)
		switch {
		case len(entries) != 1:
			findings = append(findings, fmt.Sprintf("%s has %d Concept bindings", subject, len(entries)))
		case declaration == nil:
			findings = append(findings, subject+" names no service with a declared Public interface")
		default:
			findings = append(findings, operationMappingFindings(subject, entries[0], declaration)...)
		}
	}

	return append(findings, unboundConceptFindings(concepts, bindings)...)
}

func conceptBindingEntries(bindings []architecture.ConceptBinding,
	binding architecture.ConceptBinding,
) []architecture.PhraseMapping {
	entries := make([]architecture.PhraseMapping, 0)

	for _, candidate := range bindings {
		if candidate.Concept == binding.Concept && candidate.Block == binding.Block && candidate.Service == binding.Service {
			entries = append(entries, architecture.PhraseMapping{
				References: candidate.References, Does: candidate.Does, Intent: candidate.Intent,
			})
		}
	}

	return entries
}

func unboundConceptFindings(concepts []product.Concept, bindings []architecture.ConceptBinding) []string {
	findings := make([]string, 0)

	for _, entry := range concepts {
		bound := false

		for _, binding := range bindings {
			if binding.Concept == entry.Name && binding.Block == entry.Block {
				bound = true

				break
			}
		}

		if !bound {
			findings = append(findings, fmt.Sprintf("%s concept '%s' has no Concept binding", entry.Block, entry.Name))
		}
	}

	return findings
}

func declaredConcept(concepts []product.Concept, binding architecture.ConceptBinding) bool {
	for _, entry := range concepts {
		if entry.Name == binding.Concept && entry.Block == binding.Block {
			return true
		}
	}

	return false
}

func bindingInterface(services []architecture.Service, name string) *architecture.PublicInterface {
	for _, service := range services {
		if service.Name == name {
			return service.PublicInterface
		}
	}

	return nil
}
