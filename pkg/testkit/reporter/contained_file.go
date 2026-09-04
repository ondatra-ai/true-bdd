package reporter

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// ContainedFile resolves rel underneath base and reports whether the result
// is a readable file genuinely inside base — refusing an absolute path, a
// "..", or a symlink escape. Symlinks resolve on BOTH sides.
func ContainedFile(base, rel string) (string, bool) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", false
	}

	root, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", false
	}

	resolved, err := filepath.EvalSymlinks(filepath.Join(root, rel))
	if err != nil {
		return "", false
	}

	inside, err := filepath.Rel(root, resolved)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", false
	}

	// Containment is not enough: a FIFO is perfectly contained, and
	// os.ReadFile on one blocks until a writer appears — a goroutine that
	// never returns. Only a regular file reads to EOF in bounded time.
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}

	return resolved, true
}

// ReadContained reads a file named relative to base, refusing anything that
// escapes it. The second result separates "not readable" from
// readable-and-empty, so a caller doesn't drop a zero-byte file.
func ReadContained(base, rel string) (string, bool) {
	path, ok := ContainedFile(base, rel)
	if !ok {
		return "", false
	}

	blob, err := disk.Read(path)
	if err != nil {
		return "", false
	}

	return string(blob), true
}
