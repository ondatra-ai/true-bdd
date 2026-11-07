package validate

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// stepArgumentsEvidence uses a path only when the turn already receives that architecture path.
func stepArgumentsEvidence(cfg *config.ViperConfig, path, reference string) (string, error) {
	if path == "" {
		resolved, err := docs.NewResolver(cfg).Resolve(docs.KeyArchitectureYAML)
		if errors.Is(err, docs.ErrDocPathNotConfigured) || errors.Is(err, docs.ErrDocFileMissing) {
			return "", nil
		}

		if err != nil {
			return "", fmt.Errorf("resolve step arguments: %w", err)
		}

		path = resolved
	}

	section, err := architecture.LoadStepArguments(path)
	if err != nil {
		return "", fmt.Errorf("load step arguments: %w", err)
	}

	if section == "" || reference == "" {
		return section, nil
	}

	return fmt.Sprintf("Read(`%s`) — the complete top-level `step_arguments:` section governs "+
		"how step arguments are read.\n", reference), nil
}

func (e *ChecklistEvaluator) conceptPayload(payload any) ([]byte, error) {
	section, err := e.evidence(e.config, "")
	if err != nil {
		return nil, err
	}

	if section == "" {
		return marshalConceptEvidence(payload)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode concept evidence: %w", err)
	}

	var fields map[string]json.RawMessage

	err = json.Unmarshal(encoded, &fields)
	if err != nil {
		return nil, fmt.Errorf("decode concept evidence: %w", err)
	}

	fields["step_arguments"], err = json.Marshal(section)
	if err != nil {
		return nil, fmt.Errorf("encode step arguments: %w", err)
	}

	return marshalConceptEvidence(fields)
}

func applierArchitectureReference(data FixApplierData) string {
	if documents, ok := data.Subject.(interface{ SpecificationDocuments() map[string]string }); ok {
		return documents.SpecificationDocuments()[docs.KeyArchitectureYAML]
	}

	if data.Judgement == "words" {
		return ""
	}

	if scenario, ok := data.Subject.(*template.ScenarioCheckData); ok {
		for _, document := range scenario.TermDocuments {
			if document.Key == docs.KeyArchitectureYAML {
				return document.Scratch
			}
		}
	}

	return ""
}

func marshalConceptEvidence(payload any) ([]byte, error) {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode concept payload: %w", err)
	}

	return encoded, nil
}

func specificationArchitectureReference(subject any) string {
	documents, ok := subject.(interface{ SpecificationDocuments() map[string]string })
	if !ok {
		return ""
	}

	return documents.SpecificationDocuments()[docs.KeyArchitectureYAML]
}
