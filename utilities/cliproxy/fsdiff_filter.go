package main

import (
	"github.com/ondatra-ai/true-bdd/pkg/testkit/fstree"
)

// normalizeChanges drops declared omitted paths and rewrites the rest for
// storage: run-volatile substrings ({{CWD}}, {{RUN_DIR}}) are normalized in
// both path and content, so a cassette recorded in one run dir applies inside another.
func normalizeChanges(changes []fstree.Change, cwd string, rules recording) []fstree.Change {
	out := make([]fstree.Change, 0, len(changes))

	for _, change := range changes {
		if rules.omits(change.Path) {
			continue
		}

		normalized := fstree.Change{
			Path: normalize(change.Path, cwd, rules),
			Kind: change.Kind,
		}

		if len(change.After) > 0 {
			normalized.After = []byte(normalize(string(change.After), cwd, rules))
		}

		out = append(out, normalized)
	}

	return out
}
