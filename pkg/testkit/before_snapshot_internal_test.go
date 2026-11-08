package testkit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/internal/runner"
)

const snapshotFixtureName = "sample"

func TestBeforeSnapshotRunsAfterInputAndBeforeServiceAndSnapshot(t *testing.T) {
	t.Parallel()
	state := hookScenarioRun(t)

	var err error

	err = state.PrepareTree(snapshotFixtureName)
	if err != nil {
		t.Fatal(err)
	}

	state.BeforeSnapshot(func(directory string) error {
		path := filepath.Join(directory, "source")

		before, readErr := disk.Read(path)
		if readErr != nil {
			return readErr
		}

		if string(before) != "wide" {
			t.Errorf("hook did not see assembled input: %q", before)
		}

		return disk.Write(path, []byte("narrowed"), disk.Private)
	})

	err = state.Run("exercise", RunOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if state.Result.ExitCode != 0 {
		t.Fatalf("service did not see hook output: %d", state.Result.ExitCode)
	}

	changes := state.Result.Diff
	if len(changes) != 1 || changes[0].Path != "output" || string(changes[0].After) != "produced" {
		t.Fatalf("hook entered service diff: %+v", changes)
	}
}

func hookScenarioRun(t *testing.T) *ScenarioRun {
	t.Helper()
	fixtures := t.TempDir()

	input := filepath.Join(fixtures, "sample/input")

	err := os.MkdirAll(input, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(input, "source"), []byte("wide"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	binary := filepath.Join(t.TempDir(), "service")

	script := "#!/bin/sh\n[ \"$(cat source)\" = narrowed ] || exit 9\nprintf produced > output\n"

	err = os.WriteFile(binary, []byte(script), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	shared := &harness{BinPath: binary, SessionRoot: t.TempDir(), FixturesDir: fixtures,
		FixtureStep: `fixture (.+)`, Modes: runner.Modes{Services: runner.ProxyModeLive, Tests: runner.ProxyModeReplay}}

	state, err := newScenarioRun(shared)(&bddgo.World{T: t, Scenario: bddgo.Scenario{ID: snapshotFixtureName,
		Steps: []bddgo.Step{{Text: "fixture sample"}}}})
	if err != nil {
		t.Fatal(err)
	}

	return state
}
