package steps

import (
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Saved rendered prompts count, scratch artifacts the outcome diff omits included; no match fails.
//
//truebdd:concept then "file content"
func assertEveryFileMatches(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	full, err := state.containedPath(args[0])
	if err != nil {
		return err
	}

	files, err := filepath.Glob(full)
	if err != nil {
		return state.fail("invalid glob %q: %v", args[0], err)
	}

	if len(files) == 0 {
		return state.fail("no file matches %q", args[0])
	}

	for _, file := range files {
		resolved, err := state.containedMatch(args[0], file)
		if err != nil {
			return err
		}

		pattern, err := compileFilePattern(state, file, args[2])
		if err != nil {
			return err
		}

		content, readErr := disk.Read(resolved)
		if readErr != nil {
			return state.fail("read %s: %v", file, readErr)
		}

		if pattern.Match(content) != (args[1] == "matches") {
			return state.fail("expected %s to %s %q", file, args[1], args[2])
		}
	}

	return nil
}
