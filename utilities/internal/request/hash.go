// Package request fingerprints a CLI request after caller-supplied normalization.
package request

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash separates arguments and stdin with NUL bytes before hashing normalized text.
func Hash(argv []string, stdin []byte, normalize func(string) string) string {
	hasher := sha256.New()
	for _, arg := range argv {
		_, _ = hasher.Write([]byte(normalize(arg)))
		_, _ = hasher.Write([]byte{0})
	}

	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(normalize(string(stdin))))

	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}
