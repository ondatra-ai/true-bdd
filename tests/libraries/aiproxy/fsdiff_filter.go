package main

import (
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/tests/libraries/fstree"
)

// isEngineOwned reports workdir paths that must never enter a
// cassette's fs-diff even though they change during an AI call: they
// are written by the ENGINE process concurrently with the call (its
// slog file), by the shim itself (cursor state), or are crush's
// per-turn runtime state (generated config dir + SQLite). Each exists
// independently in every mode — replaying a recorded copy would clash
// with the live run's own writes.
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
// storage: run-volatile substrings ({{CWD}}, {{RUN_DIR}}) are
// normalized in both the path AND the content, so a cassette recorded
// in one run dir can be applied inside another. Replay reverses the
// substitution against its own run (denormalizer).
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
