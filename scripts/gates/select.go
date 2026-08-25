package gates

import "strings"

// Select returns the gates a diff touching these paths needs, in table order.
// Fail-safe: a path matching no glob anywhere runs EVERYTHING, so a directory
// nobody wrote a rule for cannot slip through unchecked.
func Select(changed []string) []Gate {
	if len(changed) == 0 || hasUnknown(changed) {
		return All
	}

	var selected []Gate

	for _, gate := range All {
		if gate.Always() || gate.matches(changed) {
			selected = append(selected, gate)
		}
	}

	return selected
}

// MatchGlob reports whether a repo-relative path falls under one glob. Three
// shapes, which is every shape the table uses: "**/*.ext", "prefix/**", and a
// literal path. SupportedGlob keeps a fourth from arriving unnoticed.
func MatchGlob(glob, path string) bool {
	switch {
	case strings.HasPrefix(glob, "**/*."):
		return strings.HasSuffix(path, strings.TrimPrefix(glob, "**/*"))
	case strings.HasSuffix(glob, "/**"):
		return strings.HasPrefix(path, strings.TrimSuffix(glob, "**"))
	default:
		return path == glob
	}
}

// SupportedGlob reports whether MatchGlob understands this pattern. Anything
// else would silently match nothing, which in a fail-safe selector reads as
// "run everything" and hides the mistake.
func SupportedGlob(glob string) bool {
	switch {
	case strings.HasPrefix(glob, "**/*."):
		return !strings.ContainsAny(strings.TrimPrefix(glob, "**/*."), "*/")
	case strings.HasSuffix(glob, "/**"):
		return !strings.Contains(strings.TrimSuffix(glob, "/**"), "*")
	default:
		return !strings.Contains(glob, "*")
	}
}

// matches reports whether any changed path falls inside this gate's globs.
func (g Gate) matches(changed []string) bool {
	for _, path := range changed {
		for _, glob := range g.Globs {
			if MatchGlob(glob, path) {
				return true
			}
		}
	}

	return false
}

// hasUnknown reports whether some path matches no rule anywhere in the table.
func hasUnknown(changed []string) bool {
	for _, path := range changed {
		if !known(path) {
			return true
		}
	}

	return false
}

func known(path string) bool {
	for _, gate := range All {
		if gate.matches([]string{path}) {
			return true
		}
	}

	return false
}
