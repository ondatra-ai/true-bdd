package main

import (
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/tests/libraries/fstree"
)

// isEngineOwned reports workdir paths that must never enter a cassette's
// fs-diff: the engine's own slog file, the shim's cursor state, and crush's
// per-turn runtime state — each written independently in every mode.
func isEngineOwned(rel string) bool {
	if rel == filepath.Join("tmp", "true-bdd.log.json") {
		return true
	}

	if strings.HasPrefix(rel, filepath.Join("tmp", "aiproxy-state")+string(filepath.Separator)) {
		return true
	}

	for _, segment := range strings.Split(rel, string(filepath.Separator)) {
		if strings.HasPrefix(segment, "crush-config-") {
			return true
		}
	}

	return false
}

// normalizeChanges drops engine-owned paths and rewrites the rest for
// storage: run-volatile substrings ({{CWD}}, {{RUN_DIR}}) are normalized in
// both path and content, so a cassette recorded in one run dir applies inside another.
func normalizeChanges(changes []fstree.Change, cwd string) []fstree.Change {
	out := make([]fstree.Change, 0, len(changes))

	for _, change := range changes {
		if isEngineOwned(change.Path) {
			continue
		}

		normalized := fstree.Change{
			Path: normalizeText(change.Path, cwd),
			Kind: change.Kind,
		}

		if len(change.After) > 0 {
			normalized.After = []byte(normalizeText(string(change.After), cwd))
		}

		out = append(out, normalized)
	}

	return out
}
