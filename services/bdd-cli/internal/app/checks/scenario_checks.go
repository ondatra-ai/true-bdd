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
	// CheckTermMappings names the check that the architecture's word
	// mapping maps, exactly once, every term those phrases declare.
	CheckTermMappings = "term-mappings"
	// CheckStepDefinitions names the check that those phrases and the
	// suite's step definitions correspond one to one.
	CheckStepDefinitions = "step-definitions"
)

// ForScenarios is the Set a `scen check` Prompt may name: every built-in
// check that reads one Registry Scenario.
func ForScenarios() Set[*template.ScenarioCheckData] {
	return Set[*template.ScenarioCheckData]{
		CheckGivenWhenThen:   givenWhenThen(),
		CheckStepPhrases:     stepPhrases(),
		CheckDeclaredTerms:   declaredTerms(),
		CheckTermMappings:    termMappings(),
		CheckStepDefinitions: stepDefinitions(),
	}
}
