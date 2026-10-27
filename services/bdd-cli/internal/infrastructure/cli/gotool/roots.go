package gotool

import (
	"context"
	"fmt"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/cli/git"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/shell"
	"strings"
)

// Bin is the Go toolchain executable.
const Bin = "go"

// ReadRoots asks the installed Go toolchain for its tool and module directories.
func ReadRoots(ctx context.Context, dir string) ([]string, error) {
	result, err := shell.Run(ctx, []string{Bin, envVerb, goRootEnv, goModCacheEnv},
		shell.Options{Dir: dir, Env: git.Environment()})
	if err != nil {
		return nil, fmt.Errorf("locating Go read roots: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("locating Go read roots: %w", result.Err())
	}

	return strings.Split(strings.TrimSpace(result.Stdout), "\n"), nil
}
