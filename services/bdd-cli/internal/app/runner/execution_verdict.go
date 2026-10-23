package runner

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
)

// recordExecutionPass replaces a stale failure with the runner's fresh pass.
func recordExecutionPass[I any](
	spec Spec[I], builder *reportBuilder, item I, query *checklistmodels.PromptWithContext,
) error {
	subjectID, subjectTitle := spec.GetSubject(item)
	result := validate.CheckResult(*query, checks.Verdict{
		Pass:    true,
		Context: []string{"Test passed on the runner's fresh verdict"},
	})
	builder.Add(subjectID, subjectTitle, result)

	err := validate.SaveCheckVerdict(spec.TmpDir, subjectID, result)
	if err != nil {
		return fmt.Errorf("save execution verdict: %w", err)
	}

	return nil
}
