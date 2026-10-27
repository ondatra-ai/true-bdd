package steps

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
)

// compileFilePattern matches serialized paths literally, including format escapes and regex metacharacters.
func compileFilePattern(state *State, file, pattern string) (*regexp.Regexp, error) {
	paths, err := filePatternPaths(state, pattern)
	if err != nil {
		return nil, err
	}

	for token, alternatives := range paths {
		literals := make([]string, 0, len(alternatives))

		for _, path := range alternatives {
			if token == "{{ENGINE}}" && filepath.Ext(file) == ".json" {
				path = strings.ReplaceAll(path, "'", `'"'"'`)
			}

			literal, encodeErr := serializedPatternPath(file, path)
			if encodeErr != nil {
				return nil, state.fail("serialize pattern path: %w", encodeErr)
			}

			literals = append(literals, regexp.QuoteMeta(literal))
		}

		pattern = strings.ReplaceAll(pattern, token, "(?:"+strings.Join(literals, "|")+")")
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, state.fail("invalid pattern %q: %w", pattern, err)
	}

	return compiled, nil
}

func filePatternPaths(state *State, pattern string) (map[string][]string, error) {
	paths := make(map[string][]string)

	if strings.Contains(pattern, "{{CWD}}") {
		root, err := filepath.EvalSymlinks(state.Result.TmpDir)
		if err != nil {
			return nil, state.fail("resolve pattern root: %w", err)
		}

		paths["{{CWD}}"] = []string{root}
	}

	if strings.Contains(pattern, "{{ENGINE}}") {
		executable, err := filepath.EvalSymlinks(state.Harness.BinPath)
		if err != nil {
			return nil, state.fail("resolve pattern engine: %w", err)
		}

		paths["{{ENGINE}}"] = []string{state.Harness.BinPath, executable}
	}

	if strings.Contains(pattern, "{{GOROOT}}") || strings.Contains(pattern, "{{GOMODCACHE}}") {
		toolchain, err := gotool.Environment(state.T.Context())
		if err != nil {
			return nil, state.fail("resolve pattern toolchain: %w", err)
		}

		paths["{{GOROOT}}"] = []string{toolchain["GOROOT"]}
		paths["{{GOMODCACHE}}"] = []string{toolchain["GOMODCACHE"]}
	}

	return paths, nil
}

func serializedPatternPath(file, path string) (string, error) {
	switch filepath.Ext(file) {
	case ".json":
		encoded, err := json.Marshal(path)
		if err != nil {
			return "", fmt.Errorf("encode JSON path: %w", err)
		}

		return string(encoded[1 : len(encoded)-1]), nil
	case ".toml":
		quoted := strconv.Quote(path)

		return quoted[1 : len(quoted)-1], nil
	default:
		return path, nil
	}
}
