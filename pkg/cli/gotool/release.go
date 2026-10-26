package gotool

import (
	"context"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// BuildRelease cross-compiles the exported engine. Version stamping belongs to #220.
func BuildRelease(ctx context.Context, dir, binary, targetOS, targetArch string) error {
	env := git.Environment().Strip("GOFLAGS").Set(
		"GOOS="+targetOS, "GOARCH="+targetArch, "CGO_ENABLED=0", "GOWORK=off")

	result, err := shell.Run(ctx, []string{Bin, buildVerb, trimPathFlag, "-o", binary, enginePackage},
		shell.Options{Dir: dir, Env: env, Group: true})
	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return fmt.Errorf("building %s/%s: %w", targetOS, targetArch, err)
	}

	return nil
}

const harnessModule = "github.com/ondatra-ai/true-bdd"

// RequireHarness installs the package and its transitive dependencies, locally or from the proxy.
func RequireHarness(ctx context.Context, opt shell.Options, source, revision string) error {
	err := prepareHarnessModule(ctx, opt, source)
	if err != nil {
		return err
	}

	if source != "" {
		revision = "v0.0.0"
	}

	result, err := shell.Run(ctx, []string{Bin, "get", harnessModule + "/pkg/testkit/bddgo@" + revision}, opt)
	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return fmt.Errorf("requiring harness: %w", err)
	}

	return nil
}

func prepareHarnessModule(ctx context.Context, opt shell.Options, source string) error {
	args := []string{Bin, "mod", "edit"}
	if source != "" {
		args = append(args, "-replace="+harnessModule+"="+source)
	} else {
		args = append(args, "-droprequire="+harnessModule, "-dropreplace="+harnessModule)
	}

	result, err := shell.Run(ctx, args, opt)
	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return fmt.Errorf("preparing harness module: %w", err)
	}

	return nil
}
