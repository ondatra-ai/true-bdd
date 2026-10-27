package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

type runtimeFailureGroup struct {
	scenarios []string
	output    string
}

// runtimeFailureReport groups one verification's evidence across its selections.
type runtimeFailureReport struct {
	selections []string
	groups     []runtimeFailureGroup
	byText     map[string]int
}

func (r *runtimeFailureReport) addSelection(names, ids []string, failures []*testrunner.FailingTest) {
	if len(failures) == 0 {
		return
	}

	if r.byText == nil {
		r.byText = make(map[string]int)
	}

	r.selections = append(r.selections, "Selection Scenarios: "+strings.Join(ids, ", "))
	for _, failure := range failures {
		affected := affectedScenarios(failure.TestName, names, ids)

		identity := ""
		if len(affected) == 1 {
			identity = affected[0]
		}

		key := normalizeRuntimeFailure(failure.FailureOutput, failure.TestName, identity)

		index, exists := r.byText[key]
		if !exists {
			index = len(r.groups)
			r.byText[key] = index
			r.groups = append(r.groups, runtimeFailureGroup{output: failure.FailureOutput})
		}

		for _, id := range affected {
			if !slices.Contains(r.groups[index].scenarios, id) {
				r.groups[index].scenarios = append(r.groups[index].scenarios, id)
			}
		}
	}
}

func (r *runtimeFailureReport) render() []string {
	result := append([]string(nil), r.selections...)
	for _, group := range r.groups {
		result = append(result, fmt.Sprintf("Failure affecting Scenarios: %s\n%s",
			strings.Join(group.scenarios, ", "), group.output))
	}

	return result
}

func affectedScenarios(testName string, names, ids []string) []string {
	for index, name := range names {
		if testName == name || strings.HasPrefix(testName, name+"/") {
			return []string{ids[index]}
		}
	}
	// A package/build failure has no individual Scenario; it affects the selection.
	return ids
}
