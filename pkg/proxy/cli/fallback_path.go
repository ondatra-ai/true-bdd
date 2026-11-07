package cli

import (
	"os"
	"strings"
)

// FallbackPath places directory after the active stand-in and before the remaining PATH, or first when no stand-in
// is active.
func FallbackPath(run interface {
	Environment() []string
	SetEnv(entries ...string)
}, directory string) {
	path := os.Getenv("PATH")
	shim := ""

	for _, entry := range run.Environment() {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			path = value
		}

		if value, ok := strings.CutPrefix(entry, "TRUE_BDD_AIPROXY_DIR="); ok {
			shim = value
		}
	}

	prefix := directory + string(os.PathListSeparator)
	if shim != "" {
		path = strings.TrimPrefix(path, shim+string(os.PathListSeparator))
		prefix = shim + string(os.PathListSeparator) + prefix
	}

	run.SetEnv("PATH=" + prefix + path)
}
