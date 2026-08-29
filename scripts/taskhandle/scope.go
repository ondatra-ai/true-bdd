package taskhandle

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

// everything is what a Ticket writes when the change really is repo-wide.
// gates.MatchGlob does not understand it and gates.SupportedGlob rejects it,
// so it is normalised here rather than by widening the gate selector.
const everything = "./*"

// parseGlobs splits an Expected Changes field: one glob per line, or
// comma-separated, per ticket-schema.yaml.
func parseGlobs(field string) []string {
	var globs []string

	for _, line := range strings.FieldsFunc(field, func(r rune) bool {
		return r == '\n' || r == ',' || r == '\r'
	}) {
		glob := strings.TrimSpace(line)
		if glob != "" {
			globs = append(globs, glob)
		}
	}

	return globs
}

// outOfScope names every changed path no glob covers.
func outOfScope(changed, globs []string) []string {
	var stray []string

	for _, path := range changed {
		if !covered(path, globs) {
			stray = append(stray, path)
		}
	}

	return stray
}

func covered(path string, globs []string) bool {
	for _, glob := range globs {
		if matches(normalizeGlob(glob), path) {
			return true
		}
	}

	return false
}

// matches delegates to the gate selector's matcher for the three shapes it
// understands, and answers the repo-wide case itself.
func matches(glob, path string) bool {
	if glob == everything {
		return true
	}

	return gates.MatchGlob(glob, path)
}

// normalizeGlob turns the shapes a person writes into the three MatchGlob
// reads: a bare directory and a trailing slash both mean "everything under".
func normalizeGlob(glob string) string {
	glob = strings.TrimPrefix(strings.TrimSpace(glob), "./")

	switch {
	case glob == "" || glob == "*" || glob == "**":
		return everything
	case strings.HasSuffix(glob, "/**"), strings.HasPrefix(glob, "**/*."):
		return glob
	case strings.HasSuffix(glob, "/"):
		return glob + "**"
	case strings.Contains(glob, "*") || strings.Contains(glob, "."):
		return glob
	default:
		return glob + "/**"
	}
}
