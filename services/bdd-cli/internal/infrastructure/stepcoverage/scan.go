// Package stepcoverage reports which registry steps bind to no step
// definition, by reading the definitions themselves.
//
// The subprocess this replaced argued that source could only ever
// approximate the answer, because some patterns are assembled at
// registration rather than written as literals. The assembly is real,
// but it is CONSTANT-expression assembly — a literal joined by a
// package-level const — which a constant folder reproduces exactly.
// What made source-reading a guess was the habit of SKIPPING a pattern
// it could not read: against a suite that splices one shared selector
// into a hundred registrations, a skip reports every step they bind as
// unbound. This package never skips. Anything that does not fold is a
// refusal naming the file, the line and the expression, so the failure
// mode is a stopped run rather than a silent under-report.
//
// Reading rather than linking is not a preference: depguard's
// root-services list denies product code any import of the test tree.
//
// Every Step call in the package counts, not only those reachable from
// Register: reachability needs types a parser does not have, and
// over-collecting can only turn a gap into a binding or into an
// ambiguity refusal — never into a scenario silently missed.
package stepcoverage

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/registry"
)

// stepsDir is the directory, under a suite's own root, holding the step
// definitions for the scenarios that root owns.
const stepsDir = "steps"

// ErrTwoRootsForOneService signals a service whose scenarios name two
// suite roots, so no single steps/ dir holds its definitions.
// scenariogen.BuildPlan refuses it first; kept so order cannot matter.
var ErrTwoRootsForOneService = errors.New("a service's scenarios name two suite roots")

// Answer is what the scan found: which scenarios it examined, and which
// of them have a step that binds to nothing. Kept apart because an empty
// gap list alone means nothing — a scenario nobody examined has one too.
type Answer struct {
	Examined map[string]bool
	Gaps     map[string][]string
}

// Scan reports which of these scenarios' steps bind to no definition.
func Scan(scenarios []*registry.RegistryScenario, repoRoot string) (*Answer, error) {
	roots, err := serviceRoots(scenarios)
	if err != nil {
		return nil, err
	}

	byService, err := loadByService(roots, repoRoot)
	if err != nil {
		return nil, err
	}

	answer := &Answer{
		Examined: make(map[string]bool, len(scenarios)),
		Gaps:     map[string][]string{},
	}

	for _, scenario := range scenarios {
		gaps, gapErr := gapsFor(scenario, byService[scenario.Service])
		if gapErr != nil {
			return nil, gapErr
		}

		answer.Examined[scenario.ID] = true

		if len(gaps) > 0 {
			answer.Gaps[scenario.ID] = gaps
		}
	}

	return answer, nil
}

// serviceRoots maps each service to the directory holding its tests —
// path.Dir of a scenario's own path:, never the service's name, which
// names no directory (ADR 0009).
func serviceRoots(scenarios []*registry.RegistryScenario) (map[string]string, error) {
	roots := map[string]string{}

	for _, scenario := range scenarios {
		root := path.Dir(filepath.ToSlash(scenario.Path))

		seen, known := roots[scenario.Service]
		if !known {
			roots[scenario.Service] = root

			continue
		}

		if seen != root {
			return nil, fmt.Errorf("%s: %q: %w (%s is also in %q)",
				scenario.ID, scenario.Path, ErrTwoRootsForOneService, scenario.Service, seen)
		}
	}

	return roots, nil
}

// loadByService reads every suite's definitions before a single step is
// resolved: a pattern that will not fold must beat an ambiguity to the
// refusal whichever service holds it, or the message would vary by run.
func loadByService(roots map[string]string, repoRoot string) (map[string][]definition, error) {
	byDir := map[string][]definition{}
	byService := make(map[string][]definition, len(roots))

	for _, service := range sortedKeys(roots) {
		dir := filepath.Join(repoRoot, filepath.FromSlash(roots[service]), stepsDir)

		defs, cached := byDir[dir]
		if !cached {
			loaded, err := loadDefinitions(dir)
			if err != nil {
				return nil, err
			}

			byDir[dir] = loaded
			defs = loaded
		}

		byService[service] = defs
	}

	return byService, nil
}

// sortedKeys orders the services, so the first refusal is the same one
// on every run.
func sortedKeys(roots map[string]string) []string {
	names := make([]string, 0, len(roots))

	for name := range roots {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
