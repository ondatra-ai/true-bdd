package checks

import "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"

// CheckGivenWhenThen names the check that a Scenario carries Given, When
// and Then steps.
const CheckGivenWhenThen = "given-when-then"

// ForScenarios is the Set a `scen check` Prompt may name: every built-in
// check that reads one Registry Scenario.
func ForScenarios() Set[*template.ScenarioCheckData] {
	return Set[*template.ScenarioCheckData]{
		CheckGivenWhenThen: givenWhenThen(),
	}
}
