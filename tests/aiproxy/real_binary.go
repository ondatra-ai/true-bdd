package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const execBits = 0o111

var errRealBinaryNotFound = errors.New("real binary not found on PATH outside the shim dir")

// resolveRealBinary finds the genuine CLI behind the shim: the first
// PATH entry that is not the shim dir and holds an executable with this
// name. No hardcoded install locations — the shim dir simply loses its
// PATH priority for this one lookup.
func resolveRealBinary(name, shimDir string) (string, error) {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}

		abs, absErr := filepath.Abs(dir)
		if absErr != nil || abs == shimDir {
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
