package codex

import (
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// RuntimeExecutables locates the optional installed command runtimes, never a whole user PATH grant.
func RuntimeExecutables() map[string]string {
	paths := make(map[string]string)

	for _, name := range []string{"node", "npm", "docker"} {
		path, err := shell.Find(name)
		if err == nil {
			paths[name] = filepath.Clean(path)
		}
	}

	return paths
}
