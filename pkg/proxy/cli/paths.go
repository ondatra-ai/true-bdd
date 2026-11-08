package cli

import (
	"path"
	"path/filepath"
	"strings"
)

// Omits reports paths excluded from per-call recordings, with a double-star segment matching zero or more path
// components.
func (r Recording) Omits(relative string) bool {
	return r.SkipsSnapshot(relative) || matchesPaths(r.Unrecorded, relative)
}

// SkipsSnapshot reports paths that neither a run nor a recording may read.
func (r Recording) SkipsSnapshot(relative string) bool {
	return matchesPaths(r.Unread, relative)
}

func matchesPaths(patterns []string, relative string) bool {
	for _, pattern := range patterns {
		if matchesPath(strings.Split(filepath.ToSlash(pattern), "/"), strings.Split(filepath.ToSlash(relative), "/")) {
			return true
		}
	}

	return false
}

func matchesPath(pattern, relative []string) bool {
	if len(pattern) == 0 {
		return len(relative) == 0
	}

	if pattern[0] == "**" {
		if len(pattern) == 1 {
			return true
		}

		return matchesPath(pattern[1:], relative) || (len(relative) > 0 && matchesPath(pattern, relative[1:]))
	}

	if len(relative) == 0 {
		return false
	}

	matched, _ := path.Match(pattern[0], relative[0])

	return matched && matchesPath(pattern[1:], relative[1:])
}
