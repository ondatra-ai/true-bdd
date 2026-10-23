package steps

import (
	"path/filepath"
	"regexp"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// assertEveryFileMatches checks saved rendered prompts, including scratch
// artifacts omitted from the outcome diff. No matches is a failure.
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

	pattern, err := regexp.Compile(args[2])
	if err != nil {
		return state.fail("invalid pattern %q: %v", args[2], err)
	}

	for _, file := range files {
		resolved, err := state.containedMatch(args[0], file)
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
