package checks

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func phraseMappings() Check[*template.ScenarioCheckData] {
	return closureCheck([]string{docs.KeyProduct, docs.KeyArchitectureYAML}, reviewPhraseMappings)
}

func reviewPhraseMappings(item *template.ScenarioCheckData, input Input, source product.Catalogue) (Verdict, error) {
	path := input.Docs[docs.KeyArchitectureYAML]

	mappings, err := architecture.LoadPhraseMappings(path)
	if err != nil {
		return Verdict{}, fmt.Errorf("read Phrase mapping: %w", err)
	}

	services, err := architecture.LoadInterfaces(path, ".")
	if err != nil {
		return Verdict{}, fmt.Errorf("read Public interfaces: %w", err)
	}

	var declaration *architecture.PublicInterface

	for _, service := range services {
		if service.Name == item.Service {
			declaration = service.PublicInterface

			break
		}
	}

	findings := mappedPhraseFindings(item, source, mappings, declaration)

	if len(findings) != 0 {
		return Verdict{Pass: false, Context: prefixed(item.ID, findings)}, nil
	}

	context := item.ID + ": every matched phrase has one valid Phrase mapping for service " + item.Service

	return Verdict{Pass: true, Context: []string{context}}, nil
}

func mappedPhraseFindings(item *template.ScenarioCheckData, source product.Catalogue,
	mappings []architecture.PhraseMapping, declaration *architecture.PublicInterface,
) []string {
	compiled, _ := catalogue.Compile(source.Entries, source.Roles)
	findings := make([]string, 0)
	seen := make(map[string]bool)

	for _, step := range item.Phrases {
		if step.ModelRun() {
			continue
		}

		matches := compiled.Match(step.Text)
		if len(matches) != 1 || matches[0].Block != step.Block || seen[matches[0].Template] {
			continue
		}

		phrase := matches[0].Template
		seen[phrase] = true
		entries := mappingsFor(mappings, phrase, item.Service)

		findings = append(findings, phraseMappingFindings(phrase, item.Service, entries, declaration)...)
	}

	return findings
}

func mappingsFor(mappings []architecture.PhraseMapping, phrase, service string) []architecture.PhraseMapping {
	entries := make([]architecture.PhraseMapping, 0)

	for _, mapping := range mappings {
		if mapping.Template == phrase && mapping.Service == service {
			entries = append(entries, mapping)
		}
	}

	return entries
}

func phraseMappingFindings(phrase, service string, entries []architecture.PhraseMapping,
	declaration *architecture.PublicInterface,
) []string {
	subject := fmt.Sprintf("phrase '%s' for service %s", phrase, service)
	if len(entries) == 0 {
		return []string{subject + " has no Phrase mapping entry (reference: missing)"}
	}

	if len(entries) != 1 {
		return []string{fmt.Sprintf("%s has %d Phrase mapping entries (references: %v)", subject, len(entries), entries)}
	}

	entry := entries[0]
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

func intentFinding(subject string, entry architecture.PhraseMapping,
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
