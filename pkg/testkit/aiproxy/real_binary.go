package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const execBits = 0o111

var errRealBinaryNotFound = errors.New("real binary not found on PATH outside the shim dir")

// resolveRealBinary finds the genuine CLI behind the shim: the first PATH
// entry that is no shim dir and holds an executable of this name. EVERY
// known shim is skipped — skipping only its own finds the other and recurses.
func resolveRealBinary(name, shimDir string) (string, error) {
	skip := shimDirs(shimDir)

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}

		abs, absErr := filepath.Abs(dir)
		if absErr != nil || skip[abs] {
			continue
		}

		candidate := filepath.Join(abs, name)

		info, statErr := os.Stat(candidate)
		if statErr != nil || info.IsDir() || info.Mode()&execBits == 0 {
			continue
		}

		return candidate, nil
	}

	return "", fmt.Errorf("%w: %s", errRealBinaryNotFound, name)
}

// shimDirs is every directory a genuine binary must not be found in.
func shimDirs(own string) map[string]bool {
	skip := map[string]bool{own: true}

	for _, dir := range filepath.SplitList(os.Getenv(envKnownShims)) {
		if dir == "" {
			continue
		}

		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}

		skip[abs] = true
	}

	return skip
}
