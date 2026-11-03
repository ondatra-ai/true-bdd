package validate

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"

// modelView is a validation-only projection; templates still receive the original typed Subject.
func (data ChecklistPromptData) modelView() map[string]any {
	subject := data.Subject
	if projected, ok := subject.(interface{ ChecklistModelSubject() any }); ok {
		subject = projected.ChecklistModelSubject()
	}

	documents := make(map[string]any, len(data.Docs))
	for key, document := range data.Docs {
		documents[key] = modelDocument(document)
	}

	return map[string]any{
		"Question": data.Question, "Rationale": data.Rationale, "FixTemplate": data.FixTemplate,
		"ResultPath": data.ResultPath, "Structured": data.Structured, "Docs": documents, "Subject": subject,
	}
}

func modelDocument(document *docs.ArchitectureDoc) any {
	if document == nil {
		return nil
	}

	return map[string]any{"FilePath": document.FilePath}
}
