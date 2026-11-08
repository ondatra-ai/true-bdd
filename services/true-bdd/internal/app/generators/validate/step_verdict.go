package validate

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// CellArtifactStem gives a specialized Q the same artifact names as other checklist questions.
func CellArtifactStem(tmpDir, subjectID string, prompt checklist.PromptWithContext) string {
	return fmt.Sprintf("%s/%02d-%s-checklist-%s",
		tmpDir, prompt.Index, sanitizeID(subjectID), sanitizeID(prompt.GetFullSectionPath()))
}

// SaveStepVerdict retains the verification Q's typed answer, including named finding categories.
func SaveStepVerdict(
	tmpDir, subjectID string, prompt checklist.PromptWithContext, findings []CritiqueFinding, answer string,
) error {
	content, err := json.Marshal(struct {
		Answer   string            `json:"answer"`
		Findings []CritiqueFinding `json:"findings"`
	}{answer, findings})
	if err != nil {
		return fmt.Errorf("encode step verification: %w", err)
	}

	stem := CellArtifactStem(tmpDir, subjectID, prompt)
	for _, name := range []string{"response.txt", "result.yaml"} {
		err = disk.Write(stem+"-"+name, content, disk.Shared)
		if err != nil {
			return fmt.Errorf("save step verification: %w", err)
		}
	}

	slog.Info("Result file saved", "file", stem+"-result.yaml")

	return nil
}
