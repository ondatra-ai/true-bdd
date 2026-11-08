package gotool

import (
	"context"
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/git"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/shell"
)

// PackageDocumentation describes the complete exported API resolved by the host's module graph.
func PackageDocumentation(ctx context.Context, host, imported string) (string, error) {
	_, err := PackageDirectory(ctx, host, imported)
	if err != nil {
		return "", err
	}

	result, err := shell.Run(ctx, []string{Bin, "doc", "-all", imported}, shell.Options{
		Dir: host, Group: true, Env: git.Environment().Strip("GOFLAGS").Set("GOWORK=off", "GOTOOLCHAIN=local"),
	})
	if err != nil {
		return "", fmt.Errorf("document Go package %s: %w", imported, err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf("document Go package %s: %s: %w", imported, result.Stderr, result.Err())
	}

	return result.Stdout, nil
}
