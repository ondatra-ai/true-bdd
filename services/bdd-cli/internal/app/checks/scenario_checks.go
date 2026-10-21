package checks

import "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"

// The built-in checks a `scen check` Prompt may name.
const (
	// CheckGivenWhenThen names the check that a Scenario carries Given,
	// When and Then steps.
	CheckGivenWhenThen = "given-when-then"
	// CheckStepPhrases names the check that every step of a Scenario a
	// Story claims matches exactly one phrase of the product's catalogue.
	CheckStepPhrases = "step-phrases"
	// CheckDeclaredTerms names the check that the product glossary
	// defines, exactly once, every term those phrases declare.
	CheckDeclaredTerms = "declared-terms"
)

// ForScenarios is the Set a `scen check` Prompt may name: every built-in
// check that reads one Registry Scenario.
func ForScenarios() Set[*template.ScenarioCheckData] {
	return Set[*template.ScenarioCheckData]{
		CheckGivenWhenThen: givenWhenThen(),
		CheckStepPhrases:   stepPhrases(),
		CheckDeclaredTerms: declaredTerms(),
	}
}
