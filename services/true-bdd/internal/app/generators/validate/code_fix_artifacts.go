package validate

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// SaveCodeFix retains a code F's actual input and rendered menu prompt without a model turn.
func SaveCodeFix(tmpDir, subjectID string, failed checklist.ValidationResult, prompt string, iteration int) error {
	if tmpDir == "" {
		return nil
	}

	prefix := fmt.Sprintf("%02d-%s-fix", failed.PromptIndex, sanitizeID(subjectID))
	input := fmt.Sprintf("### Failed Check: %s\n\n**Question:** %s\n\n**Actual:** %s\n\n%s\n\n%s\n",
		failed.SectionPath, failed.Question, failed.ActualAnswer, strings.Join(failed.Context, "\n"), failed.FixPrompt)

	files := map[string]string{
		fmt.Sprintf("%s-iter%d-user.txt", prefix, iteration): input,
		prefix + "-prompts.md":                               prompt + "\n",
	}
	for name, content := range files {
		err := disk.Write(filepath.Join(tmpDir, name), []byte(content), disk.Shared)
		if err != nil {
			return fmt.Errorf("save code fix: %w", err)
		}
	}

	slog.Info("Generating fix prompt", "subjectID", subjectID, "promptIndex", failed.PromptIndex)

	return nil
}

// SaveCodeApply records a completed code F in the ordinary apply audit format.
func SaveCodeApply(tmpDir, subjectID string, iteration int) error {
	if tmpDir == "" {
		return nil
	}

	name := fmt.Sprintf("apply-%s-iter%d-result.yaml", sanitizeID(subjectID), iteration)

	err := disk.Write(filepath.Join(tmpDir, name), []byte("applied: true\n"), disk.Shared)
	if err != nil {
		return fmt.Errorf("save code apply: %w", err)
	}

	slog.Info("Fix applied successfully", "subjectID", subjectID)

	return nil
}
