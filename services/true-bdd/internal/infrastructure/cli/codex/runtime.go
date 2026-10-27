package codex

import (
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/shell"
)

// RuntimeExecutables locates the optional installed command runtimes, never a whole user PATH grant.
func RuntimeExecutables() map[string]string {
	paths := make(map[string]string)

	for _, name := range []string{"node", "npm", "npx", "docker"} {
		path, err := shell.Find(name)
		if err == nil {
			paths[name] = filepath.Clean(path)
		}
	}

	return paths
}
