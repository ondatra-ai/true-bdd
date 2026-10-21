package gates_test

import (
	"os"
	"testing"
)

// TestMain drops the repository-locating variables git exports to a hook (`git rev-parse --local-env-vars`):
// under the pre-commit hook in a linked worktree they made these tests' temp-repo `git init`/`config`/`add`
// write the real repo's config (`core.bare = true`, a `t@example.com` user) and index (spec #127, ticket #128).
func TestMain(m *testing.M) {
	for _, name := range []string{
		"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
		"GIT_OBJECT_DIRECTORY", "GIT_DIR", "GIT_WORK_TREE", "GIT_IMPLICIT_WORK_TREE", "GIT_GRAFT_FILE",
		"GIT_INDEX_FILE", "GIT_NO_REPLACE_OBJECTS", "GIT_REPLACE_REF_BASE", "GIT_PREFIX", "GIT_SHALLOW_FILE",
		"GIT_COMMON_DIR",
	} {
		_ = os.Unsetenv(name)
	}

	os.Exit(m.Run())
}
