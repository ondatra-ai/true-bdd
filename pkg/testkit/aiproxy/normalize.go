package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"strings"
)

const (
	cwdPlaceholder    = "{{CWD}}"
	runDirPlaceholder = "{{RUN_DIR}}"
	homePlaceholder   = "{{HOME}}"

	// runDirPattern matches the engine's run-directory basename — timestamp-pid
	// (fs/run_directory.go), which differs every run and would otherwise
	// make every request hash unstable.
	runDirPattern = `\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}-\d+`
)

// runDirRe is compiled once. Compiling it per call cost nothing when
// normalization ran over argv alone, and costs real time now that every
// stdout line goes through it.
var runDirRe = regexp.MustCompile(runDirPattern)

// normalizeText replaces the run-volatile substrings a call can embed — cwd,
// the engine's per-run tmp dir, and the recording machine's home (see
// TestNormalizeReplacesHome for why home matters) — with stable placeholders.
func normalizeText(text, cwd string) string {
	text = strings.ReplaceAll(text, cwd, cwdPlaceholder)

	text = replaceHome(text)

	return runDirRe.ReplaceAllString(text, runDirPlaceholder)
}

// replaceHome substitutes the home directory only where it is a whole path
// component (exact, or followed by a separator) — a bare substring replace
// would also mangle a sibling like /Users/peterson (TestNormalizeLeavesSiblingPathsAlone).
func replaceHome(text string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || home == string(os.PathSeparator) {
		return text
	}

	home = strings.TrimSuffix(home, string(os.PathSeparator))

	text = strings.ReplaceAll(text, home+string(os.PathSeparator),
		homePlaceholder+string(os.PathSeparator))

	// The bare path, only where nothing path-like follows it: a trailing
	// occurrence, or one before a character that cannot continue a
	// component.
	return regexp.MustCompile(regexp.QuoteMeta(home)+`(?:$|(?P<after>[^\w./-]))`).
		ReplaceAllString(text, homePlaceholder+"${after}")
}

// normalizeArgv normalizes every argument for storage in meta.json.
func normalizeArgv(argv []string, cwd string) []string {
	out := make([]string, len(argv))
	for i, arg := range argv {
		out[i] = normalizeText(arg, cwd)
	}

	return out
}

// requestHash fingerprints one call: sha256 over the normalized argv
// (NUL-joined) and the normalized stdin. Replay compares it against the
// cassette's hash, so an edit since recording fails loudly instead of replaying stale.
func requestHash(argv []string, stdin []byte, cwd string) string {
	hasher := sha256.New()

	for _, arg := range argv {
		_, _ = hasher.Write([]byte(normalizeText(arg, cwd)))
		_, _ = hasher.Write([]byte{0})
	}

	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(normalizeText(string(stdin), cwd)))

	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}
