package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

type linkedBinding struct {
	Concept product.Concept             `json:"concept"`
	Binding architecture.ConceptBinding `json:"binding"`
}

func (e *ChecklistEvaluator) evaluateBindings(ctx context.Context, subject any, prompt checklist.PromptWithContext,
	source product.Document, result checklist.ValidationResult,
) (checklist.ValidationResult, error) {
	item, ok := subject.(*template.ScenarioCheckData)
	if !ok || !item.ConceptLinkPassed {
		failConcept(&result, "binding judgement requires this walk's successful link")
		e.saveConceptResult(result)

		return result, nil
	}

	bindings, services, err := e.loadBindingDocuments()
	if err != nil {
		return result, err
	}

	linked, findings := scenarioBindings(item, source.Concepts, bindings)
	switch {
	case len(findings) > 0:
		for _, finding := range findings {
			failConcept(&result, finding)
		}
	case len(linked) > 0:
		err = e.judgeBindings(ctx, prompt, item.Service, linked, services, conceptSteps(subject), &result)
		if err != nil {
			return result, err
		}
	default:
		result.Context = []string{"no linked concepts require a binding"}
	}

	e.saveConceptResult(result)

	return result, nil
}

func (e *ChecklistEvaluator) judgeBindings(ctx context.Context, prompt checklist.PromptWithContext,
	service string, linked map[string]linkedBinding, services []architecture.Service,
	steps []conceptStep, result *checklist.ValidationResult,
) error {
	interfaceContext, err := conceptInterfaceContext(services, service)
	if err != nil {
		return err
	}

	productText, err := e.conceptProduct()
	if err != nil {
		return err
	}

	payload := struct {
		Product         string                   `json:"product"`
		Steps           []conceptStep            `json:"steps"`
		Service         string                   `json:"service"`
		Bindings        map[string]linkedBinding `json:"bindings"`
		PublicInterface map[string]any           `json:"public_interface"`
	}{productText, steps, service, linked, interfaceContext}

	schema, err := bindingSchema(linked)
	if err != nil {
		return err
	}

	response, err := e.conceptTurn(ctx, prompt, payload, schema)
	if err != nil {
		return err
	}

	gradeBindings(response, linked, result)

	return nil
}

func scenarioBindings(item *template.ScenarioCheckData, concepts []product.Concept,
	bindings []architecture.ConceptBinding,
) (map[string]linkedBinding, []string) {
	linked := make(map[string]linkedBinding)
	findings := make([]string, 0)

	for _, link := range item.ConceptLinks {
		key := string(link.Block) + ":" + link.Concept

		matches := matchingConceptBindings(bindings, link, item.Service)
		if len(matches) != 1 {
			findings = append(findings, fmt.Sprintf("%s concept %q for service %s has %d Concept bindings",
				link.Block, link.Concept, item.Service, len(matches)))

			continue
		}

		if link.Intent && strings.TrimSpace(matches[0].Intent) == "" {
			findings = append(findings, key+": llm: step requires a binding stated by intent")

			continue
		}

		linked[key] = linkedBinding{linkedConcept(concepts, link), matches[0]}
	}

	return linked, findings
}

func matchingConceptBindings(bindings []architecture.ConceptBinding, link template.ConceptLink,
	service string,
) []architecture.ConceptBinding {
	matches := make([]architecture.ConceptBinding, 0)

	for _, binding := range bindings {
		if binding.Concept == link.Concept && binding.Block == link.Block && binding.Service == service {
			matches = append(matches, binding)
		}
	}

	return matches
}

func linkedConcept(concepts []product.Concept, link template.ConceptLink) product.Concept {
	for _, concept := range concepts {
		if concept.Name == link.Concept && concept.Block == link.Block {
			return concept
		}
	}

	return product.Concept{}
}

func bindingSchema(linked map[string]linkedBinding) (string, error) {
	properties := answerSchema{}
	for key := range linked {
		properties[key] = requiredObject(answerSchema{"unambiguous": answerSchema{schemaType: "boolean"},
			schemaReason: answerSchema{schemaType: schemaString, schemaMinLength: 1}})
	}

	encoded, err := json.Marshal(requiredObject(answerSchema{"bindings": requiredObject(properties)}))
	if err != nil {
		return "", fmt.Errorf("encode binding schema: %w", err)
	}

	return string(encoded), nil
}

func gradeBindings(response string, linked map[string]linkedBinding, result *checklist.ValidationResult) {
	var answer struct {
		Bindings map[string]struct {
			Unambiguous *bool  `json:"unambiguous"`
			Reason      string `json:"reason"`
		} `json:"bindings"`
	}

	err := strictAnswer(response, &answer)
	if err != nil {
		failConcept(result, "binding answer malformed or incomplete: "+err.Error())

		return
	}

	for _, key := range sortedBindingKeys(linked) {
		verdict, ok := answer.Bindings[key]
		if !ok || verdict.Unambiguous == nil || strings.TrimSpace(verdict.Reason) == "" {
			failConcept(result, key+": binding answer missing or incomplete")

			continue
		}

		delete(answer.Bindings, key)

		result.Context = append(result.Context, key+": "+verdict.Reason)
		if !*verdict.Unambiguous {
			failConcept(result, key+": binding is ambiguous")
		}
	}

	if len(answer.Bindings) > 0 {
		failConcept(result, "binding answer contains unknown concepts")
	}
}

func (e *ChecklistEvaluator) loadBindingDocuments() ([]architecture.ConceptBinding, []architecture.Service, error) {
	path, err := e.docResolver.Resolve(docs.KeyArchitectureYAML)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve concept architecture: %w", err)
	}

	bindings, err := architecture.LoadConceptBindings(path)
	if err != nil {
		return nil, nil, fmt.Errorf("load concept bindings: %w", err)
	}

	services, err := architecture.LoadInterfaces(path, ".")
	if err != nil {
		return nil, nil, fmt.Errorf("load concept interfaces: %w", err)
	}

	return bindings, services, nil
}
