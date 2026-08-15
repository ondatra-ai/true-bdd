//go:build bdd

package bdd_test

import (
	"os"
	"path/filepath"
	"testing"
)

// stagingDirName holds record mode's in-flight cassettes, one
// subdirectory per fixture, at the session root.
//
// At the session root and not inside the fixture's own run directory:
// cassettes are written DURING the run, and everything written inside
// the run directory lands in the post-run snapshot and therefore in the
// prompt the judge grades. A hundred kilobytes of recorded prompts in
// that diff would drown the handful of files the fixture is actually
// about.
//
// The reporter counts a session subdirectory as a fixture only when it
// holds an engine log or a harness record, so this one is invisible to
// the report while still sitting next to the run it belongs to.
const stagingDirName = ".cassettes-staging"

// cassetteDir is a fixture's committed cassette directory in the source
// tree — what replay reads. fixtureDir is the fixture's folder under
// tests/bdd-cli/fixtures/.
func cassetteDir(t *testing.T, fixtureDir string) string {
	t.Helper()

	abs, err := filepath.Abs(filepath.Join(fixtureDir, "cassettes"))
	if err != nil {
		t.Fatalf("resolve cassettes dir: %v", err)
	}

	return abs
}

// prepareStaging gives a record-mode run an empty directory to write
// into. Empty rather than appended-to, so a shorter re-recording cannot
// leave a longer one's trailing calls behind.
func prepareStaging(t *testing.T, sessionRoot, name string) string {
	t.Helper()

	staging := filepath.Join(sessionRoot, stagingDirName, name)

	err := os.RemoveAll(staging)
	if err != nil {
		t.Fatalf("clear staging cassettes: %v", err)
	}

	err = os.MkdirAll(staging, 0o755)
	if err != nil {
		t.Fatalf("create staging cassettes: %v", err)
	}

	return staging
}

// promoteCassettes publishes a recording into the source tree. Called
// only for a fixture that PASSED: a cassette set is a recording of
// correct behaviour, and one taken from a failed run would make replay
// reproduce the failure forever — green in record mode today, red in
// replay mode tomorrow, with the evidence already overwritten.
//
// Leaving a failed run's cassettes in staging is the other half of that:
// the previous, good cassettes are still in place, and the rejected ones
// are still on disk to look at.
func promoteCassettes(t *testing.T, fixtureDir, staging string) {
	t.Helper()

	dest := cassetteDir(t, fixtureDir)

	err := os.RemoveAll(dest)
	if err != nil {
		t.Fatalf("clear cassettes dir: %v", err)
	}

	err = os.Rename(staging, dest)
	if err != nil {
		t.Fatalf("publish cassettes to %s: %v", dest, err)
	}

	// No keep file is needed: golden.json is written for every recording,
	// including a fixture that spawns no AI CLI at all, so the directory
	// is never empty and always survives a clone.
	t.Logf("cassettes recorded: %s", dest)
}
