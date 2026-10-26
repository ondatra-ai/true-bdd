package ai

import (
	"os"
	"path/filepath"
	"strings"
)

// resolveThroughSymlinks fails closed when a path cannot be resolved.
func resolveThroughSymlinks(path string) string {
	resolved, err := resolveWriterPath(path)
	if err != nil {
		return ""
	}

	return resolved
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
