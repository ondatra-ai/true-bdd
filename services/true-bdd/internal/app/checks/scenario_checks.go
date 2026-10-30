package checks

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"

// The built-in checks a `scen check` Prompt may name.
const (
	// CheckGivenWhenThen names the check that a Scenario carries Given,
	// When and Then steps.
	CheckGivenWhenThen = "given-when-then"
	// CheckDeclaredTerms names the check that the product glossary
	// defines, exactly once, every term the product's concepts declare.
	CheckDeclaredTerms = "declared-terms"
	// CheckTermMappings names the check that the architecture's word
	// mapping maps, exactly once, every term the product's concepts declare.
	CheckTermMappings = "term-mappings"
	// CheckConceptBindings names the check that every concept binding names
	// a declared concept and the Public interface its service declares.
	CheckConceptBindings = "concept-bindings"
	// CheckPublicInterface names the service declaration check.
	CheckPublicInterface = "public-interface"
	// CheckStepDefinitions names the check that each step binds one step
	// definition, which records a declared concept of the step's block.
	CheckStepDefinitions = "step-definitions"
)

// ForScenarios is the Set a `scen check` Prompt may name: every built-in
// check that reads one Registry Scenario.
func ForScenarios() Set[*template.ScenarioCheckData] {
	return Set[*template.ScenarioCheckData]{
		CheckGivenWhenThen:   givenWhenThen(),
		CheckDeclaredTerms:   declaredTerms(),
		CheckTermMappings:    termMappings(),
		CheckConceptBindings: conceptBindings(),
		CheckPublicInterface: publicInterface(),
		CheckStepDefinitions: stepDefinitions(),
	}
}
