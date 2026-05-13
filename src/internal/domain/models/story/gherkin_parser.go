package story

import (
	"fmt"

	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

// GherkinParser creates TestScenario entries from acceptance criteria steps.
type GherkinParser struct{}

// GenerateScenarios creates TestScenario entries from acceptance criteria.
// Each AC with Steps populated produces one TestScenario. ACs without Steps return an error.
func (p *GherkinParser) GenerateScenarios(storyID string, acs []AcceptanceCriterion) ([]TestScenario, error) {
	var scenarios []TestScenario

	seqNum := 0

	for _, criterion := range acs {
		if len(criterion.Steps) == 0 {
			return nil, pkgerrors.ErrACMissingStepsError(criterion.ID)
		}

		seqNum++

		scenario := TestScenario{
			ID:                 fmt.Sprintf("%s-%03d", storyID, seqNum),
			AcceptanceCriteria: []string{criterion.ID},
			Steps:              criterion.Steps,
			Level:              "",
			Priority:           "",
			Service:            "",
		}

		scenarios = append(scenarios, scenario)
	}

	return scenarios, nil
}
