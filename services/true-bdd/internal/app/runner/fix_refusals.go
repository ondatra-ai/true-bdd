package runner

import (
	"fmt"
	"sync"

	checklistmodels "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
)

// fixRefusals holds, per Item and Prompt, why its last fix was not
// taken, until that Prompt answers again.
type fixRefusals struct {
	mu   sync.Mutex
	held map[string]string
}

func newFixRefusals() *fixRefusals {
	return &fixRefusals{held: make(map[string]string)}
}

func (r *fixRefusals) hold(subjectID string, promptIndex int, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.held[refusalKey(subjectID, promptIndex)] = reason
}

// attach adds the refusal held for the result's Prompt to the findings of a
// result that fails again, for the next fix turn; either way it is spent.
func (r *fixRefusals) attach(subjectID string, result checklistmodels.ValidationResult) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := refusalKey(subjectID, result.PromptIndex)
	reason, held := r.held[key]
	delete(r.held, key)

	if !held || result.Status == checklistmodels.StatusPass {
		return result.Context
	}

	return append(append([]string(nil), result.Context...), reason)
}

func refusalKey(subjectID string, promptIndex int) string {
	return fmt.Sprintf("%s\x00%d", subjectID, promptIndex)
}
