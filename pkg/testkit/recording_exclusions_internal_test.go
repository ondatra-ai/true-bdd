package testkit

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
)

func TestRunDiffObservesRecordingOnlyExclusionsWithoutReadingPrivatePaths(t *testing.T) {
	t.Parallel()
	state := hookScenarioRun(t)

	state.harness.Dependencies = []proxycli.Dependency{{Name: "vendor", Recording: proxycli.Recording{
		Unrecorded: []string{"tmp/observed.log", "tmp/observations/**"},
		Unread:     []string{"tmp/private/**"},
	}}}

	err := state.PrepareTree("sample")
	if err != nil {
		t.Fatal(err)
	}

	state.BeforeSnapshot(func(root string) error {
		private := filepath.Join(root, "tmp/private")

		err := os.MkdirAll(private, 0o700)
		if err != nil {
			return fmt.Errorf("create unread directory: %w", err)
		}
		// Snapshot refuses symlinks: success proves it never traverses this directory.
		err = os.Symlink("unread-target", filepath.Join(private, "before"))
		if err != nil {
			return fmt.Errorf("plant unread link: %w", err)
		}

		return nil
	})

	binary := filepath.Join(t.TempDir(), "service")

	script := "#!/bin/sh\nset -eu\nmkdir -p tmp/observations\n" +
		"printf noise > .DS_Store\nprintf noise > tmp/.DS_Store\nprintf log > tmp/observed.log\n" +
		"printf observed > tmp/observations/event\nln -s unread-target tmp/private/after\n"

	err = os.WriteFile(binary, []byte(script), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	err = state.Run("exercise", RunOptions{Binary: binary})
	if err != nil {
		t.Fatal(err)
	}

	paths := make([]string, 0, len(state.Result.Diff))
	for _, change := range state.Result.Diff {
		paths = append(paths, change.Path)
	}

	if !slices.Equal(paths, []string{"tmp/observations/event", "tmp/observed.log"}) {
		t.Fatalf("run observations lost or private paths read: %v", paths)
	}
}
