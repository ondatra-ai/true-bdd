package ai

import "path/filepath"

// allowsReadFrom confines local reads to the working directory. An unnamed
// read uses crush's project default; relative paths are rooted there too.
func (p CrushGuardPolicy) allowsReadFrom(targetPath string) (bool, string) {
	if targetPath == "" {
		return true, ""
	}

	root := resolveThroughSymlinks(p.ReadRoot)

	target := targetPath
	if !filepath.IsAbs(target) {
		// Keep .. until symlinks resolve; cleaning first can hide an escape (#187).
		target = root + string(filepath.Separator) + target
	}

	resolved := resolveThroughSymlinks(target)
	if pathWithinRoot(resolved, root) {
		return true, ""
	}

	return false, "read target " + resolved + " is outside read root " + p.ReadRoot
}
