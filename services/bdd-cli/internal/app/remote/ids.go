package remote

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const idBytes = 16

// randomID returns a fresh random hex id (used for the per-process
// incarnation id — plan §3.3).
func randomID() (string, error) {
	buffer := make([]byte, idBytes)

	_, err := rand.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}

	return hex.EncodeToString(buffer), nil
}
