package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var errRunDirUnresolved = errors.New(
	"cassette references {{RUN_DIR}} but no engine run directory exists under tmp/ yet")

// denormalize maps a stored cassette string back onto the live run:
// {{CWD}} becomes the current working directory, {{HOME}} the replaying
// machine's home directory, and {{RUN_DIR}} the engine's current
// run-directory name.
func denormalize(text, cwd, runDir string) (string, error) {
	text = strings.ReplaceAll(text, cwdPlaceholder, cwd)

	// The replaying machine's own home, not the recording one's — that
	// is the whole point of storing a placeholder: a cassette recorded
	// under /Users/alice replays unchanged under /home/bob.
	home, homeErr := os.UserHomeDir()
	if homeErr == nil && home != "" {
		text = strings.ReplaceAll(text, homePlaceholder, home)
	}

	if strings.Contains(text, runDirPlaceholder) {
		if runDir == "" {
			return "", errRunDirUnresolved
		}

		text = strings.ReplaceAll(text, runDirPlaceholder, runDir)
	}

	return text, nil
}

// findCurrentRunDir locates the engine's run directory for THIS run —
// the newest tmp/ entry matching the timestamp-pid naming
// (fs/run_directory.go). A fixture tmpdir starts empty, so normally
// exactly one exists; newest-wins covers reruns inside one tmpdir.
// Empty result means the engine has not created one yet — only an
// error if a cassette actually needs the mapping.
func findCurrentRunDir(cwd string) string {
	entries, err := os.ReadDir(filepath.Join(cwd, "tmp"))
	if err != nil {
		return ""
	}

	pattern := regexp.MustCompile("^" + runDirPattern + "$")

	var (
		newestName string
		newestTime time.Time
	)

	for _, entry := range entries {
		if !entry.IsDir() || !pattern.MatchString(entry.Name()) {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}

		if newestName == "" || info.ModTime().After(newestTime) {
			newestName = entry.Name()
			newestTime = info.ModTime()
		}
	}

	return newestName
}
