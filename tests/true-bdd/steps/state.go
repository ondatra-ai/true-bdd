// Package steps holds TrueBDD's Scenario definitions and their own state.
package steps

import (
	"fmt"
	"github.com/ondatra-ai/true-bdd/pkg/testkit"
)

// State adds definition-owned data to the harness's Scenario run.
type State struct {
	*testkit.ScenarioRun

	guardCall *guardCall
}

var _ testkit.Judgeable = (*State)(nil)

// fail preserves wrapped sentinels while naming the Scenario.
//
//nolint:err113 // callers supply the observation and wrapped errors.
func (s *State) fail(format string, args ...any) error {
	return fmt.Errorf("%s: "+format, append([]any{s.Scenario.ID}, args...)...)
}
