package reportserver

import (
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/reporter"
)

// turnKey is the identity a turn is aligned on when two runs are
// compared: which checklist cell it belongs to, and which of the cell's
// three roles it is.
//
// The cell's ITERATION INDEX is deliberately excluded. A cell is
// retried until it passes, so the same cell legitimately produces
// several turns — us-create-happy-path runs format×2, who×1, what×2,
// why×4 — and including the index would make attempt 2 of `format`
// unequal to attempt 2 of `format` in the other run only because the
// numbering shifted. Leaving it out lets the diff algorithm do its job:
// a run that needed one extra retry yields a single Insert inside that
// cell, and every later cell still matches.
//
// Turn.Number is not usable either: it is the engine's monotonic
// counter, so keying on it is ordinal pairing wearing a different hat.
func turnKey(turn *reporter.Turn) string {
	section := turn.Cell.Section
	if section == "" {
		// A turn whose artifacts never named a section (the fix and apply
		// turns of a cell, before inheritance fills them in) still aligns
		// on its subject. Falling back to the role alone would make every
		// such turn interchangeable.
		section = "?"
	}

	subject := turn.Cell.Subject
	if subject == "" {
		subject = "?"
	}

	return strings.Join([]string{section, subject, turn.Role}, "\x1f")
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

// filePaths projects a run's file changes onto the identity they should
// be aligned on.
//
// Two normalisations, both load-bearing. First, the path alone rather
// than the rendered "created foo.yaml (809 bytes)" line — otherwise a
// file that merely changed size reports as one deletion plus one
// insertion instead of one changed row.
//
// Second, the volatile directory segments are folded away. Every prompt
// artifact lives under a scratch directory named for the run's start
// time, so comparing raw paths makes ALL of them unique to their run and
// the file diff degenerates into "everything deleted, everything added"
// — technically true and completely useless. Folding the timestamp lets
// the same artifact line up across runs, which is the only way the diff
// answers "did this run write different files?".
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
