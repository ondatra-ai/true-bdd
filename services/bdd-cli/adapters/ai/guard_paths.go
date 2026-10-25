package ai

import (
	"os"
	"path/filepath"
	"strings"
)

// resolveThroughSymlinks resolves the deepest existing ancestor and
// re-appends what does not exist, preserving .. until links resolve.
// A target need not exist yet, so EvalSymlinks on the full path can fail.
func resolveThroughSymlinks(path string) string {
	remainder := ""
	current := path

	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(resolved, remainder)
		}

		parent, leaf := filepath.Split(current)

		parent = strings.TrimSuffix(parent, string(os.PathSeparator))
		if parent == "" {
			parent = string(os.PathSeparator)
		}

		if parent == current {
			// Reached the root without finding anything that exists;
			// the lexical form is all there is.
			return filepath.Clean(path)
		}

		remainder = filepath.Join(leaf, remainder)
		current = parent
	}
}

// absoluteGlobRoot reduces a glob to the absolute directory prefix it can
// never escape (everything before the first wildcard), so `/repo/tmp/`
// can't prefix-match a sibling like `/repo/tmpfoo`.
func absoluteGlobRoot(glob, workDir string) string {
	fixed := glob

	if idx := strings.IndexAny(fixed, "*?["); idx >= 0 {
		fixed = fixed[:idx]
	}

	fixed = strings.TrimSpace(fixed)
	if fixed == "" {
		return ""
	}

	if !filepath.IsAbs(fixed) {
		fixed = filepath.Join(workDir, fixed)
	}

	return filepath.Clean(fixed) + string(os.PathSeparator)
}
