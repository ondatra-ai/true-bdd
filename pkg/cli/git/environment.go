package git

import (
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// localEnvVars is exactly `git rev-parse --local-env-vars` (issue #206).
func localEnvVars() []string {
	return []string{
		"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
		"GIT_OBJECT_DIRECTORY", "GIT_DIR", "GIT_WORK_TREE", "GIT_IMPLICIT_WORK_TREE", "GIT_GRAFT_FILE",
		"GIT_INDEX_FILE", "GIT_NO_REPLACE_OBJECTS", "GIT_REPLACE_REF_BASE", "GIT_PREFIX", "GIT_SHALLOW_FILE",
		"GIT_COMMON_DIR",
	}
}

// Environment inherits everything except Git's repository-local variables.
// Hooks export these; inheriting them redirects commands away from their cwd.
func Environment() shell.Env {
	return shell.Inherit().Strip(localEnvVars()...)
}

// ScrubEnvironment applies the same policy to explicit subprocess environments.
func ScrubEnvironment(entries []string) []string {
	return slices.DeleteFunc(slices.Clone(entries), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")

		return slices.Contains(localEnvVars(), name)
	})
}
