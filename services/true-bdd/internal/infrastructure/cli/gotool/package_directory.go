package gotool

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/git"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/shell"
)

var errPackageDirectory = errors.New("go package directory is not an absolute path")

// PackageDirectory resolves an import through the host's module graph without editing its manifest.
func PackageDirectory(ctx context.Context, host, imported string) (string, error) {
	result, err := shell.Run(ctx, []string{Bin, "list", "-mod=readonly", "-find", "-f={{.Dir}}", imported}, shell.Options{
		Dir: host, Group: true, Env: git.Environment().Strip("GOFLAGS").Set("GOWORK=off", "GOTOOLCHAIN=local"),
	})
	if err != nil {
		return "", fmt.Errorf("resolve Go package %s: %w", imported, err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf("resolve Go package %s: %s: %w", imported, result.Stderr, result.Err())
	}

	directory := strings.TrimSpace(result.Stdout)
	if !filepath.IsAbs(directory) || strings.ContainsAny(directory, "\r\n") {
		return "", fmt.Errorf("resolve Go package %s returned %q: %w", imported, directory, errPackageDirectory)
	}

	return filepath.Clean(directory), nil
}
