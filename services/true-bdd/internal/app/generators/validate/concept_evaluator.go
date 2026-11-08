package validate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

func (e *ChecklistEvaluator) evaluateConcepts(ctx context.Context, subject any, subjectID string,
	prompt checklist.PromptWithContext, kind checklist.ConceptJudgement,
) (checklist.ValidationResult, error) {
	path, err := e.docResolver.Resolve(docs.KeyProduct)
	if err != nil {
		return checklist.ValidationResult{}, fmt.Errorf("resolve link product: %w", err)
	}

	source, err := product.Load(path)
	if err != nil {
		return checklist.ValidationResult{}, fmt.Errorf("read link product: %w", err)
	}

	result := conceptResult(prompt, kind)

	if item, ok := subject.(*template.ScenarioCheckData); ok {
		item.ConceptsDeclared = source.ConceptsDeclared
	}

	if !source.ConceptsDeclared {
		result.Context = []string{subjectID + ": not checked: the product document declares no concepts"}

		return result, nil
	}

	return e.judgeConcepts(ctx, subject, prompt, kind, source, result)
}

// judgeConcepts answers the judgement on a concept host.
func (e *ChecklistEvaluator) judgeConcepts(ctx context.Context, subject any, prompt checklist.PromptWithContext,
	kind checklist.ConceptJudgement, source product.Document, result checklist.ValidationResult,
) (checklist.ValidationResult, error) {
	switch kind {
	case checklist.JudgementGroup:
		return e.evaluateGroup(ctx, subject, prompt, source, result)
	case checklist.JudgementPropose:
		return e.evaluatePropose(ctx, subject, prompt, source, result)
	case checklist.JudgementWords:
		return e.evaluateWords(ctx, subject, prompt, source, result)
	case checklist.JudgementBindings:
		return e.evaluateBindings(ctx, subject, prompt, source, result)
	case checklist.JudgementLink:
		return e.evaluateLink(ctx, subject, prompt, source, result)
	case checklist.JudgementSketch:
	}

	return checklist.ValidationResult{}, fmt.Errorf("concept judgement %q: %w", kind, errConceptAnswer)
}

func conceptResult(prompt checklist.PromptWithContext, kind checklist.ConceptJudgement) checklist.ValidationResult {
	return checklist.ValidationResult{SectionPath: prompt.GetFullSectionPath(), Question: prompt.Prompt.Question,
		Judgement: kind, ActualAnswer: AnswerPass, Status: checklist.StatusPass, Rationale: prompt.Prompt.Rationale,
		FixPrompt: strings.TrimSpace(prompt.Prompt.FixTemplate), PromptIndex: prompt.Index, Docs: prompt.GetEffectiveDocs(),
		FixModelTier: prompt.EffectiveFixTier(), ApplyModelTier: prompt.EffectiveApplyTier()}
}

func (e *ChecklistEvaluator) conceptTurn(ctx context.Context, prompt checklist.PromptWithContext,
	payload any, schema string,
) (string, error) {
	model, err := e.models.ResolveRole(provider.RolePrompt, prompt.EffectiveModelTier())
	if err != nil {
		return "", fmt.Errorf("resolve concept model: %w", err)
	}

	encoded, err := e.conceptPayload(payload)
	if err != nil {
		return "", fmt.Errorf("encode concept context: %w", err)
	}

	system := prompt.Prompt.Question + "\nAnswer only the JSON object described by the supplied schema. Change nothing.\n"
	user := string(encoded)
	section := prompt.GetFullSectionPath()
	e.savePromptFile(section, prompt.Index, "system", system)
	e.savePromptFile(section, prompt.Index, "user", user)

	response, err := e.aiClient.ExecutePromptWithSystem(ctx, provider.RolePrompt, system, user, model,
		e.modeFactory.GetThinkMode(), schema, provider.TurnOptions{ArtifactLabel: e.artifactLabel})
	if errors.Is(err, pkgerrors.ErrClaudeStreamNoOutput) || errors.Is(err, pkgerrors.ErrProviderNoOutput) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("concept validation turn: %w", err)
	}

	e.savePromptFile(section, prompt.Index, "model-answer", response)

	return response, nil
}

func (e *ChecklistEvaluator) saveConceptResult(result checklist.ValidationResult) {
	slog.Info("Concept judgement answered", "subjectID", e.subjectID, "promptIndex", result.PromptIndex,
		"judgement", result.Judgement, "answer", result.ActualAnswer, "context", result.Context)
	path := cellArtifactPath(e.tmpDir, result.PromptIndex, e.subjectID, result.SectionPath, "result.yaml")

	encoded, err := json.Marshal(struct {
		Answer    string   `json:"answer"`
		Context   []string `json:"context"`
		FixPrompt string   `json:"fix_prompt,omitempty"`
	}{result.ActualAnswer, result.Context, result.FixPrompt})
	if err != nil {
		slog.Warn("Cannot encode concept verdict", "error", err)

		return
	}

	e.savePromptFile(result.SectionPath, result.PromptIndex, "response", string(encoded))
	e.saveResultArtifact(path, string(encoded))
}

func failConcept(result *checklist.ValidationResult, reason string) {
	result.Status = checklist.StatusFail
	result.ActualAnswer = AnswerFail
	result.Context = append(result.Context, reason)
}

func (e *ChecklistEvaluator) conceptProduct() (string, error) {
	path, err := e.docResolver.Resolve(docs.KeyProduct)
	if err != nil {
		return "", fmt.Errorf("resolve concept product: %w", err)
	}

	content, err := disk.Read(path)
	if err != nil {
		return "", fmt.Errorf("read concept product: %w", err)
	}

	return string(content), nil
}
