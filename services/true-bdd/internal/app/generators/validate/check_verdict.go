package validate

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// checkAnswer is a check's verdict as a schema-bearing model turn writes
// it, so the run dir and the coverage census read both kinds alike.
type checkAnswer struct {
	Answer  string   `json:"answer"`
	Context []string `json:"context"`
}

// CheckResult builds a `D:` Cell's result from its check's verdict. On a
// fail the prompt's own `F:` is the fix prompt, which a model would copy.
func CheckResult(promptCtx checklist.PromptWithContext, verdict checks.Verdict) checklist.ValidationResult {
	answer, status, fixPrompt := AnswerPass, checklist.StatusPass, ""
	if !verdict.Pass {
		answer, status = AnswerFail, checklist.StatusFail
		fixPrompt = strings.TrimSpace(promptCtx.Prompt.FixTemplate)
	}

	return checklist.ValidationResult{
		SectionPath:    promptCtx.GetFullSectionPath(),
		Question:       promptCtx.Prompt.Label(),
		Check:          promptCtx.Prompt.Check,
		ActualAnswer:   answer,
		Context:        verdict.Context,
		Status:         status,
		Rationale:      promptCtx.Prompt.Rationale,
		FixPrompt:      fixPrompt,
		PromptIndex:    promptCtx.Index,
		Docs:           promptCtx.GetEffectiveDocs(),
		FixModelTier:   promptCtx.EffectiveFixTier(),
		ApplyModelTier: promptCtx.EffectiveApplyTier(),
	}
}

// SkippedResult is the report row of a `Q:` Cell a failed check spared,
// its question shown as the row of an answered one would show it.
func SkippedResult(promptCtx checklist.PromptWithContext) checklist.ValidationResult {
	return checklist.ValidationResult{
		SectionPath: promptCtx.GetFullSectionPath(),
		Question:    promptCtx.Prompt.Question,
		Status:      checklist.StatusSkip,
		Rationale:   promptCtx.Prompt.Rationale,
		PromptIndex: promptCtx.Index,
		Docs:        promptCtx.GetEffectiveDocs(),
	}
}

// SaveCheckVerdict writes a `D:` Cell's verdict as the response and result
// artifacts a model Cell leaves, under the same names.
func SaveCheckVerdict(tmpDir, subjectID string, result checklist.ValidationResult) error {
	if tmpDir == "" {
		return nil
	}

	content, err := json.Marshal(checkAnswer{Answer: result.ActualAnswer, Context: result.Context})
	if err != nil {
		return fmt.Errorf("encode check verdict: %w", err)
	}

	var path string

	// Result last, and the only one logged: the census reads that record
	// as the Cell's final save, as it does for a model Cell.
	for _, name := range []string{"response.txt", "result.yaml"} {
		path = cellArtifactPath(tmpDir, result.PromptIndex, subjectID, result.SectionPath, name)

		err = disk.Write(path, content, disk.Shared)
		if err != nil {
			return fmt.Errorf("save check verdict: %w", err)
		}
	}

	slog.Info("Result file saved", "file", path)

	return nil
}

// cellArtifactPath names one Cell's artifact in the run's tmp dir. The
// result path is rendered into the model's prompt, so replay hashes it.
func cellArtifactPath(tmpDir string, promptIndex int, subjectID, sectionPath, name string) string {
	return fmt.Sprintf("%s/%02d-%s-checklist-%s-%s",
		tmpDir, promptIndex, sanitizeID(subjectID), sanitizeID(sectionPath), name)
}
