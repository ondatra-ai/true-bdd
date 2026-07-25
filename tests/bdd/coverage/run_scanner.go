package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// FixtureRun is one fixture working directory under
// <root>/<session>/<fixture>/ as prepared and retained by the BDD harness.
type FixtureRun struct {
	Session    string
	Fixture    string
	Dir        string
	ConfigDir  string   // path of the copied engine config dir ("" if absent)
	LogPath    string   // slog JSON file ("" if absent)
	Partitions []string // per-invocation artifact dirs tmp/<YYYY-MM-DD-HH-MM>
}

var partitionRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-\d{2}-\d{2}$`)

// configDirNames covers the current and the pre-rename config dir
// name, both present among retained runs.
func configDirNames() []string { return []string{"true-bdd", "bdd-cli"} }

// logFileNames covers the current and the pre-rename slog file name.
func logFileNames() []string { return []string{"true-bdd.log.json", "bdd-cli.log.json"} }

// ScanRoot discovers every fixture run directory under root, sorted by
// session then fixture.
func ScanRoot(root string) ([]FixtureRun, error) {
	sessions, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading run root %s: %w", root, err)
	}

	runs := make([]FixtureRun, 0)

	for _, session := range sessions {
		if !session.IsDir() {
			continue
		}

		sessionDir := filepath.Join(root, session.Name())

		fixtures, readErr := os.ReadDir(sessionDir)
		if readErr != nil {
			return nil, fmt.Errorf("reading session %s: %w", sessionDir, readErr)
		}

		for _, fixture := range fixtures {
			if !fixture.IsDir() {
				continue
			}

			runs = append(runs, scanFixtureDir(session.Name(), fixture.Name(),
				filepath.Join(sessionDir, fixture.Name())))
		}
	}

	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Session != runs[j].Session {
			return runs[i].Session < runs[j].Session
		}

		return runs[i].Fixture < runs[j].Fixture
	})

	return runs, nil
}

// scanFixtureDir locates the config dir, log file, and artifact
// partitions inside one fixture working directory.
func scanFixtureDir(session, fixture, dir string) FixtureRun {
	run := FixtureRun{Session: session, Fixture: fixture, Dir: dir}
	run.ConfigDir = firstExistingDir(dir, configDirNames())

	tmpDir := filepath.Join(dir, "tmp")
	run.LogPath = firstExistingFile(tmpDir, logFileNames())
	run.Partitions = partitionDirs(tmpDir)

	return run
}

// firstExistingDir returns the first candidate subdirectory that exists.
func firstExistingDir(parent string, names []string) string {
	for _, name := range names {
		candidate := filepath.Join(parent, name)

		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate
		}
	}

	return ""
}

// firstExistingFile returns the first candidate file that exists.
func firstExistingFile(parent string, names []string) string {
	for _, name := range names {
		candidate := filepath.Join(parent, name)

		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}

	return ""
}

// partitionDirs lists the timestamped artifact dirs under tmp/.
func partitionDirs(tmpDir string) []string {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil
	}

	partitions := make([]string, 0)

	for _, entry := range entries {
		if entry.IsDir() && partitionRe.MatchString(entry.Name()) {
			partitions = append(partitions, filepath.Join(tmpDir, entry.Name()))
		}
	}

	sort.Strings(partitions)

	return partitions
}
