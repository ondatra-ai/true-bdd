package steps

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
)

// compileFilePattern substitutes literal paths, so regex metacharacters in a run root cannot weaken an assertion.
func compileFilePattern(state *State, pattern string) (*regexp.Regexp, error) {
	if strings.Contains(pattern, "{{CWD}}") {
		root, err := filepath.EvalSymlinks(state.Result.TmpDir)
		if err != nil {
			return nil, state.fail("resolve pattern root: %w", err)
		}

		pattern = strings.ReplaceAll(pattern, "{{CWD}}", regexp.QuoteMeta(root))
	}

	if strings.Contains(pattern, "{{ENGINE}}") {
		executable, err := filepath.EvalSymlinks(state.Harness.BinPath)
		if err != nil {
			return nil, state.fail("resolve pattern engine: %w", err)
		}

		enginePattern := "(?:" + regexp.QuoteMeta(state.Harness.BinPath) + "|" + regexp.QuoteMeta(executable) + ")"
		pattern = strings.ReplaceAll(pattern, "{{ENGINE}}", enginePattern)
	}

	if strings.Contains(pattern, "{{GOROOT}}") || strings.Contains(pattern, "{{GOMODCACHE}}") {
		paths, err := gotool.Environment(state.T.Context())
		if err != nil {
			return nil, state.fail("resolve pattern toolchain: %w", err)
		}

		pattern = strings.NewReplacer("{{GOROOT}}", regexp.QuoteMeta(paths["GOROOT"]),
			"{{GOMODCACHE}}", regexp.QuoteMeta(paths["GOMODCACHE"])).Replace(pattern)
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, state.fail("invalid pattern %q: %w", pattern, err)
	}

	return compiled, nil
}
