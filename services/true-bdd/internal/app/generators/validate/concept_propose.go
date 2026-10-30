package validate

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

type proposalRefusal struct {
	Patterns []catalogue.ProposedPattern `json:"patterns"`
	Findings []string                    `json:"findings"`
}

type proposalRequest struct {
	Suite               string                      `json:"suite"`
	Service             string                      `json:"service"`
	Block               catalogue.Block             `json:"block"`
	Concept             map[string]any              `json:"concept"`
	Glossary            []map[string]string         `json:"glossary"`
	Binding             architecture.ConceptBinding `json:"binding"`
	PublicInterface     map[string]any              `json:"public_interface"`
	ExistingDefinitions []string                    `json:"existing_definitions"`
	Refused             *proposalRefusal            `json:"refused,omitempty"`
	Steps               []string                    `json:"steps"`
}

func (e *ChecklistEvaluator) evaluatePropose(ctx context.Context, subject any, prompt checklist.PromptWithContext,
	source product.Catalogue, result checklist.ValidationResult,
) (checklist.ValidationResult, error) {
	item, ok := subject.(*template.ConceptProposalData)
	if !ok {
		return result, errConceptAnswer
	}

	item.Patterns = nil

	request, err := e.proposalRequest(item, source)
	if err != nil {
		return result, err
	}

	response, err := e.conceptTurn(ctx, prompt, request, proposalSchema)
	if err != nil {
		return result, err
	}

	patterns, err := gradeProposal(response)
	if err != nil {
		return result, fmt.Errorf("pattern proposal malformed or incomplete (crash): %w", err)
	}

	findings := catalogue.CheckPatterns(item.ID, patterns, item.Suites)
	if len(findings) > 0 {
		for _, finding := range findings {
			failConcept(&result, finding)
		}

		item.Previous, item.Findings = patterns, findings
	} else {
		item.Patterns = patterns
		result.Context = proposalBindings(item, patterns)
	}

	e.saveConceptResult(result)

	return result, nil
}

func (e *ChecklistEvaluator) proposalRequest(item *template.ConceptProposalData, source product.Catalogue,
) (proposalRequest, error) {
	bindings, services, err := e.loadBindingDocuments()
	if err != nil {
		return proposalRequest{}, err
	}

	link := template.ConceptLink{Concept: item.Concept, Block: item.Block}

	matches := matchingConceptBindings(bindings, link, item.Service)
	if len(matches) != 1 {
		return proposalRequest{}, fmt.Errorf("%s concept %q for service %s has %d Concept bindings: %w",
			item.Block, item.Concept, item.Service, len(matches), errConceptAnswer)
	}

	interfaceContext, err := conceptInterfaceContext(services, item.Service)
	if err != nil {
		return proposalRequest{}, err
	}

	concept := linkedConcept(source.Concepts, link)

	glossary, err := e.conceptGlossary(concept.Terms)
	if err != nil {
		return proposalRequest{}, err
	}

	request := proposalRequest{Suite: item.Suite, Service: item.Service, Block: item.Block,
		Concept:  map[string]any{"concept": concept.Name, "means": concept.Means, "terms": concept.Terms},
		Glossary: glossary, Binding: matches[0], PublicInterface: interfaceContext,
		ExistingDefinitions: append([]string{}, item.Existing...), Steps: item.Steps}
	if len(item.Findings) > 0 || len(item.Critique) > 0 {
		request.Refused = &proposalRefusal{Patterns: item.Previous,
			Findings: append(append([]string{}, item.Findings...), item.Critique...)}
	}

	return request, nil
}

func (e *ChecklistEvaluator) conceptGlossary(terms []string) ([]map[string]string, error) {
	path, err := e.docResolver.Resolve(docs.KeyProduct)
	if err != nil {
		return nil, fmt.Errorf("resolve proposal glossary: %w", err)
	}

	definitions, err := product.LoadGlossary(path)
	if err != nil {
		return nil, fmt.Errorf("read proposal glossary: %w", err)
	}

	entries := make([]map[string]string, 0, len(terms))

	for _, definition := range definitions {
		if slices.Contains(terms, definition.Term) {
			entries = append(entries, map[string]string{"term": definition.Term, "means": definition.Means})
		}
	}

	return entries, nil
}

func gradeProposal(response string) ([]catalogue.ProposedPattern, error) {
	var answer struct {
		Patterns []catalogue.ProposedPattern `json:"patterns"`
	}

	err := strictAnswer(response, &answer)
	if err != nil {
		return nil, err
	}

	if len(answer.Patterns) == 0 {
		return nil, errConceptAnswer
	}

	for _, pattern := range answer.Patterns {
		if strings.TrimSpace(pattern.Pattern) == "" || pattern.Captures == nil || strings.TrimSpace(pattern.Serves) == "" {
			return nil, errConceptAnswer
		}

		for _, capture := range pattern.Captures {
			if strings.TrimSpace(capture.Name) == "" {
				return nil, errConceptAnswer
			}
		}
	}

	return answer.Patterns, nil
}

// proposalBindings logs and records what each accepted pattern captures from each step it binds.
func proposalBindings(item *template.ConceptProposalData, patterns []catalogue.ProposedPattern) []string {
	context := make([]string, 0, len(item.Steps))

	for _, pattern := range patterns {
		for _, step := range item.Steps {
			captures := catalogue.PatternCaptures(pattern.Pattern, step)
			if captures == nil {
				continue
			}

			slog.Info("Concept pattern binds", "item", item.ID, "suite", item.Suite, "pattern", pattern.Pattern,
				"step", step, "captures", strings.Join(captures, " | "))
			context = append(context, fmt.Sprintf("%s binds %q capturing %q", pattern.Pattern, step, captures))
		}
	}

	return context
}

const proposalSchema = `{"type":"object","additionalProperties":false,"required":["patterns"],"properties":{
"patterns":{"type":"array","minItems":1,"items":{"type":"object","additionalProperties":false,
"required":["pattern","captures","serves"],"properties":{
"pattern":{"type":"string","minLength":1},
"captures":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["name","means"],
"properties":{"name":{"type":"string","minLength":1},"means":{"type":"string","minLength":1}}}},
"serves":{"type":"string","minLength":1}}}}}}`
