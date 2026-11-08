package validate

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

const cannotFixField = "cannot_fix"

// cannotFixInstruction advertises the same optional answer to providers without a result schema and to writers.
const cannotFixInstruction = "\nFix only what the failed check found; " +
	"a problem it did not find is neither yours to fix nor a reason for cannot_fix. " +
	"Only if the Specification and your permitted paths leave no way to make this fix, " +
	"change nothing and return the result block with cannot_fix: " +
	"{reason: <why>, correction: <what a person must correct>}. " +
	"A failure of your own tools, sandbox or test run is no reason for cannot_fix: " +
	"make the fix and report that failure in summary."

// cannotFixFromContent refuses malformed or mixed outcomes before an obstruction can stop the walk.
func cannotFixFromContent(content string, impossibleReport bool) error {
	fields, parsed := outcomeFields(content)
	if !parsed {
		return nil
	}

	value, present := fields[cannotFixField]

	if !present || value.Tag == "!!null" {
		return nil
	}

	err := refuseMixedOutcome(fields, impossibleReport)
	if err != nil {
		return err
	}

	var report pkgerrors.CannotFixReport

	err = value.Decode(&report)
	if err != nil {
		return fmt.Errorf("%w: %w", pkgerrors.ErrCannotFixMalformed, err)
	}

	return report.Refusal()
}

// refuseMixedOutcome separates work from refusals while allowing reports alongside either.
func refuseMixedOutcome(fields map[string]yaml.Node, impossibleReport bool) error {
	for name, field := range fields {
		switch name {
		case cannotFixField, "summary", "target", "interface_cannot_serve":
			continue
		case "impossible":
			if impossibleReport {
				continue
			}
		}

		if outcomeHasWork(field) {
			return fmt.Errorf("%w: %s excludes ordinary work (%s)", pkgerrors.ErrCannotFixMalformed, cannotFixField, name)
		}
	}

	return nil
}

// outcomeFields distinguishes outcome mappings from ordinary prose and document bodies.
func outcomeFields(content string) (map[string]yaml.Node, bool) {
	var fields map[string]yaml.Node

	err := yaml.Unmarshal([]byte(stripMarkdownFences(content)), &fields)
	if err != nil {
		return nil, false
	}

	return fields, true
}

// Empty placeholders and applied:false report no ordinary work alongside a refusal.
func outcomeHasWork(node yaml.Node) bool {
	switch node.Tag {
	case "", "!!null":
		return false
	case "!!str":
		return strings.TrimSpace(node.Value) != ""
	case "!!bool":
		return node.Value != "false"
	case "!!seq", "!!map":
		return len(node.Content) != 0
	default:
		return true
	}
}
