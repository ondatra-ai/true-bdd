package steps

import (
	"path/filepath"
	"regexp"

	"github.com/ondatra-ai/true-bdd/pkg/testkit"
)

//truebdd:concept then "file content"
func assertSomeFileMatches(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	pattern, err := regexp.Compile(args[2])
	if err != nil {
		return state.fail("file pattern %q: %w", args[2], err)
	}

	var files []string

	for _, glob := range args[:2] {
		matches, err := filepath.Glob(filepath.Join(state.Result.TmpDir, glob))
		if err != nil {
			return state.fail("file glob %q: %w", glob, err)
		}

		files = append(files, matches...)
	}

	for _, file := range files {
		full, err := state.containedMatch(args[0]+" or "+args[1], file)
		if err != nil {
			return err
		}

		content, err := testkit.Read(full)
		if err != nil {
			return state.fail("read matched file: %w", err)
		}

		if pattern.Match(content) {
			return nil
		}
	}

	return state.fail("no file matching %q or %q matches %q", args[0], args[1], args[2])
}
