package steps

import (
	"path/filepath"
	"regexp"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// assertFileMatchCount counts occurrences in one user-visible file chosen by glob.
func assertFileMatchCount(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	want, err := numberSlot(args[1])
	if err != nil {
		return state.fail("match count %q: %w", args[1], err)
	}

	pattern, err := regexp.Compile(args[2])
	if err != nil {
		return state.fail("file-match pattern %q: %w", args[2], err)
	}

	matches, err := filepath.Glob(filepath.Join(state.Result.TmpDir, args[0]))
	if err != nil {
		return state.fail("file glob %q: %w", args[0], err)
	}

	if len(matches) != 1 {
		return state.fail("expected one file matching %q, got %d", args[0], len(matches))
	}

	full, err := state.containedMatch(args[0], matches[0])
	if err != nil {
		return err
	}

	content, err := disk.Read(full)
	if err != nil {
		return state.fail("read matched file %q: %w", args[0], err)
	}

	got := len(pattern.FindAllIndex(content, -1))
	if got != want {
		return state.fail("expected %d matches for %q in %q, got %d", want, args[2], args[0], got)
	}

	return nil
}
