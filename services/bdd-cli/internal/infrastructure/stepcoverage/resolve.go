package stepcoverage

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/registry"
)

// ErrAmbiguousStep signals a step matched by two definitions. Refused
// rather than reported as a gap: which one runs depends on registration
// order, and no fix turn should paper over that by adding a third.
var ErrAmbiguousStep = errors.New("step matches more than one definition")

// gapsFor reports the scenario's steps that bind to no definition,
// mirroring the suite's own resolver: a model-run step binds by
// construction, one match binds, and two refuse the whole answer.
func gapsFor(scenario *registry.RegistryScenario, defs []definition) ([]string, error) {
	var gaps []string

	for _, statement := range scenario.Statements {
		if statement.Mode != registry.ModeDeterministic {
			continue
		}

		// bddgo trims before it classifies, so the trimmed text is what
		// the suite would have matched against.
		text := strings.TrimSpace(statement.Text)
		matched := matching(text, defs)

		switch {
		case len(matched) == 0:
			gaps = append(gaps, statement.Keyword+" "+text)

		case len(matched) > 1:
			return nil, fmt.Errorf("%s: %q: %w (%s)",
				scenario.ID, text, ErrAmbiguousStep, strings.Join(matched, " | "))
		}
	}

	return gaps, nil
}

// matching is every definition whose pattern matches the step text.
// Anchoring is the pattern author's business, exactly as in the suite.
func matching(text string, defs []definition) []string {
	var sources []string

	for _, def := range defs {
		if def.pattern.FindStringSubmatch(text) != nil {
			sources = append(sources, def.source)
		}
	}

	return sources
}
