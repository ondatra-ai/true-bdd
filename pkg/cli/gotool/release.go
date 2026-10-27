package gotool

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// BuildRelease cross-compiles the exported engine. Version stamping belongs to #220.
func BuildRelease(ctx context.Context, dir, binary, targetOS, targetArch string) error {
	env := git.Environment().Strip("GOFLAGS").Set(
		"GOOS="+targetOS, "GOARCH="+targetArch, "CGO_ENABLED=0", "GOWORK=off")

	result, err := shell.Run(ctx, []string{Bin, buildVerb, trimPathFlag, "-o", binary, enginePackage},
		shell.Options{Dir: filepath.Join(dir, engineDirectory), Env: env, Group: true})
	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return fmt.Errorf("building %s/%s: %w", targetOS, targetArch, err)
	}

	return nil
}

const harnessModule = "github.com/ondatra-ai/true-bdd/pkg"

// RequireHarness installs the proxy-served package and returns its resolved exact version.
func RequireHarness(ctx context.Context, opt shell.Options, revision string) (string, error) {
	for _, args := range [][]string{
		{Bin, modVerb, "edit", "-droprequire=" + harnessModule, "-dropreplace=" + harnessModule},
		{Bin, "get", harnessModule + "/testkit/bddgo@" + revision},
	} {
		result, err := shell.Run(ctx, args, opt)
		if err == nil {
			err = result.Err()
		}

		if err != nil {
			return "", fmt.Errorf("requiring harness: %s %s: %w", result.Stdout, result.Stderr, err)
		}
	}

	result, err := shell.Run(ctx, []string{Bin, "list", "-m", "-f={{.Version}}", harnessModule}, opt)
	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return "", fmt.Errorf("resolving harness: %w", err)
	}

	return strings.TrimSpace(result.Stdout), nil
}
