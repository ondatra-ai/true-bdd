package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
)

const (
	cwdPlaceholder    = "{{CWD}}"
	runDirPlaceholder = "{{RUN_DIR}}"
	homePlaceholder   = "{{HOME}}"
	goRootPlaceholder = "{{GOROOT}}"

	// runDirPattern matches the engine's run-directory basename — timestamp-pid
	// (fs/run_directory.go), which differs every run and would otherwise
	// make every request hash unstable.
	runDirPattern = `\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}-\d+`
)

// runDirRe is compiled once. Compiling it per call cost nothing when
// normalization ran over argv alone, and costs real time now that every
// stdout line goes through it.
var runDirRe = regexp.MustCompile(runDirPattern)

//nolint:gochecknoglobals // immutable lazy discovery, shared by all normalized streams in one shim process.
var installedGoRoot = sync.OnceValues(func() (string, error) {
	paths, err := gotool.Environment(context.Background())
	if err != nil {
		return "", err
	}

	return paths["GOROOT"], nil
})

// normalizeText replaces the run-volatile substrings a call can embed — cwd,
// the engine's per-run tmp dir, home and Go installation — with stable
// placeholders, including toolchain paths embedded in compiler diagnostics.
func normalizeText(text, cwd string) string {
	text = replacePath(text, goRoot(), goRootPlaceholder)
	text = strings.ReplaceAll(text, cwd, cwdPlaceholder)

	text = replaceHome(text)

	return runDirRe.ReplaceAllString(text, runDirPlaceholder)
}

func goRoot() string {
	if root := os.Getenv("GOROOT"); root != "" {
		return root
	}

	root, _ := installedGoRoot()

	return root
}

// replaceHome substitutes the home directory only where it is a whole path
// component (exact, or followed by a separator) — a bare substring replace
// would also mangle a sibling like /Users/peterson (TestNormalizeLeavesSiblingPathsAlone).
func replaceHome(text string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return text
	}

	return replacePath(text, home, homePlaceholder)
}

func replacePath(text, path, placeholder string) string {
	path = strings.TrimSuffix(path, string(os.PathSeparator))
	if path == "" {
		return text
	}

	text = strings.ReplaceAll(text, path+string(os.PathSeparator),
		placeholder+string(os.PathSeparator))

	// The bare path, only where nothing path-like follows it: a trailing
	// occurrence, or one before a character that cannot continue a
	// component.
	return regexp.MustCompile(regexp.QuoteMeta(path)+`(?:$|(?P<after>[^\w./-]))`).
		ReplaceAllString(text, placeholder+"${after}")
}

// normalizeArgv normalizes every argument for storage in meta.json.
func normalizeArgv(argv []string, cwd string) []string {
	out := make([]string, len(argv))
	for i, arg := range stableClaudeWriterArgs(argv) {
		out[i] = normalizeText(arg, cwd)
	}

	return out
}

// requestHash fingerprints one call: sha256 over the normalized argv
// (NUL-joined) and the normalized stdin. Replay compares it against the
// cassette's hash, so an edit since recording fails loudly instead of replaying stale.
func requestHash(argv []string, stdin []byte, cwd string) string {
	hasher := sha256.New()

	for _, arg := range stableClaudeWriterArgs(argv) {
		_, _ = hasher.Write([]byte(normalizeText(arg, cwd)))
		_, _ = hasher.Write([]byte{0})
	}

	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(normalizeText(string(stdin), cwd)))

	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

// The writer supplies GOROOT, GOMODCACHE, then its exact spill root. The
// executable is not part of argv; home paths are normalized by normalizeText.
func stableClaudeWriterArgs(argv []string) []string {
	if !slices.Contains(argv, "--restricted") {
		return argv
	}

	index := slices.Index(argv, "--session-id")
	if index < 0 || index+1 >= len(argv) {
		return argv
	}

	session := argv[index+1]
	result := slices.Clone(argv)

	result[index+1] = "{{CLAUDE_SESSION}}"
	goRoots := []string{"{{GOROOT}}", "{{GOMODCACHE}}"}
	rootIndex := 0

	for idx := 1; idx < len(result); idx++ {
		if result[idx-1] != "--add-dir" {
			continue
		}

		if strings.HasSuffix(result[idx], "/"+session+"/tool-results") {
			result[idx] = "{{CLAUDE_SPILLS}}"
		} else if rootIndex < len(goRoots) {
			result[idx] = goRoots[rootIndex]
			rootIndex++
		}
	}

	return result
}
