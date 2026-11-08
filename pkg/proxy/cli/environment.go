package cli

import ("os"; "path/filepath"; "strings")

// Environment enables a caller's stand-ins, placing them first on PATH and removing other callers' stand-ins.
func Environment(mode, shimDir, cassettesDir, stateDir string, knownShims []string) []string {
	path := shimDir + string(os.PathListSeparator) + scrubShims(os.Getenv("PATH"), knownShims)

	return []string{
		"PATH=" + path,
		"TRUE_BDD_AIPROXY_MODE=" + mode,
		"TRUE_BDD_AIPROXY_CASSETTES=" + cassettesDir,
		"TRUE_BDD_AIPROXY_STATE=" + stateDir,
		"TRUE_BDD_AIPROXY_DIR=" + shimDir,
		"TRUE_BDD_AIPROXY_KNOWN_SHIMS" + "=" + strings.Join(knownShims, string(os.PathListSeparator)),
	}
}

// scrubShims drops every known shim dir from a PATH.
func scrubShims(path string, knownShims []string) string {
	if len(knownShims) == 0 {
		return path
	}

	drop := make(map[string]bool, len(knownShims))
	for _, dir := range knownShims {
		drop[absOrSelf(dir)] = true
	}

	kept := make([]string, 0, len(filepath.SplitList(path)))

	for _, dir := range filepath.SplitList(path) {
		if dir != "" && !drop[absOrSelf(dir)] {
			kept = append(kept, dir)
		}
	}

	return strings.Join(kept, string(os.PathListSeparator))
}

// absOrSelf normalizes a PATH entry for comparison.
func absOrSelf(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}

	return abs
}

