package reportserver

import (
	"regexp"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/reporter"
)

// turnKey is the identity a turn is aligned on when comparing runs: see
// Turn.CellRoleKey (reporter package, the single definition) for why the
// iteration index is excluded. Turn.Number is NOT usable: an engine counter.
func turnKey(turn *reporter.Turn) string {
	return turn.CellRoleKey()
}

// turnKeys projects a turn list for diffing.
func turnKeys(turns []*reporter.Turn) []string {
	keys := make([]string, 0, len(turns))
	for _, turn := range turns {
		keys = append(keys, turnKey(turn))
	}

	return keys
}

// testKeys projects a run's fixtures onto their names, which are unique
// within a session.
func testKeys(tests []*reporter.Fixture) []string {
	keys := make([]string, 0, len(tests))
	for _, fixture := range tests {
		keys = append(keys, fixture.Name)
	}

	return keys
}

// runScratchDir matches the engine's per-run scratch directory, whose
// name embeds the run's start timestamp and pid
// (tmp/2026-08-10-21-44-26-90237/…).
var runScratchDir = regexp.MustCompile(`^tmp/\d{4}(-\d{2}){5}-\d+/`)

// crushConfigDir matches the crush wrapper's generated config directory,
// whose name embeds a per-invocation nonce.
var crushConfigDir = regexp.MustCompile(`crush-config-\d+/`)

// filePaths projects a run's file changes onto the identity they should be
// aligned on. Two normalisations, both load-bearing:
//
//	path only    not the rendered "created foo.yaml (809 bytes)" line, or a
//	             size-only change reports as delete+insert, not one change
//	dir folding  scratch/config dirs embed a per-run timestamp/nonce, so raw
//	             paths would make every artifact unique to its run and the
//	             diff degenerate into "everything deleted, everything added"
func filePaths(files []FileChangeDTO) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, normalisePath(file.Path))
	}

	return paths
}

// normalisePath folds per-run directory names to a stable placeholder.
// Only used for alignment; the real path is always what gets displayed.
func normalisePath(path string) string {
	path = runScratchDir.ReplaceAllString(path, "tmp/<run>/")

	return crushConfigDir.ReplaceAllString(path, "crush-config-<n>/")
}
