package checks

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// conceptBindings passes when every binding names a declared concept once per
// block and service, with references its service declares and what is done
// through them, or a UI service's intent.
func conceptBindings() Check[*template.ScenarioCheckData] {
	return closureCheck([]string{docs.KeyProduct, docs.KeyArchitectureYAML},
		func(item *template.ScenarioCheckData, input Input, source product.Document) (Verdict, error) {
			return reviewConceptBindings(item, input, source.Concepts)
		})
}

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
			findings = append(findings, bindingUseFindings(subject, entries[0], declaration)...)
		}
	}

	return findings
}

func conceptBindingEntries(bindings []architecture.ConceptBinding,
	binding architecture.ConceptBinding,
) []architecture.ConceptBinding {
	entries := make([]architecture.ConceptBinding, 0)

	for _, candidate := range bindings {
		if candidate.Concept == binding.Concept && candidate.Block == binding.Block && candidate.Service == binding.Service {
			entries = append(entries, candidate)
		}
	}

	return entries
}

// bindingUseFindings holds one binding's use of its service: references
// the Public interface declares and what is done through them, or the intent
// an llm: step performs through a UI.
func bindingUseFindings(subject string, entry architecture.ConceptBinding,
	declaration *architecture.PublicInterface,
) []string {
	if strings.TrimSpace(entry.Intent) != "" {
		return intentFinding(subject, entry, declaration)
	}

	if strings.TrimSpace(entry.Does) == "" {
		return []string{fmt.Sprintf("%s requires a does sentence (references: %v)", subject, entry.References)}
	}

	if len(entry.References) == 0 {
		return []string{subject + " requires Public interface references (reference: missing)"}
	}

	findings := make([]string, 0)

	for _, reference := range entry.References {
		if !declaration.Declares(reference) {
			finding := fmt.Sprintf("%s references %s, which its Public interface does not declare", subject, reference)
			findings = append(findings, finding)
		}
	}

	return findings
}

func intentFinding(subject string, entry architecture.ConceptBinding,
	declaration *architecture.PublicInterface,
) []string {
	if entry.Does != "" || len(entry.References) != 0 {
		return []string{subject + " reference intent must replace does and references"}
	}

	if declaration == nil || declaration.Kind != "ui" {
		return []string{subject + " reference intent is not supported for a non-UI service until llm: steps are supported"}
	}

	return nil
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
