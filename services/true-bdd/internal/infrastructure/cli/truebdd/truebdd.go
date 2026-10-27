// Package truebdd resolves the engine's own executable for its hidden hook.
// Callers cannot choose another binary.
package truebdd

import (
	"fmt"
	"os"
)

// Binary is which true-bdd a spawn runs: always this process's own executable.
type Binary struct{}

// Self is the running executable — the only binary the engine spawns.
func Self() Binary {
	return Binary{}
}

// Path resolves the binary a spawn runs.
func (Binary) Path() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating this executable: %w", err)
	}

	return self, nil
}
