package gotool

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Stamp is the build identity a release binary reports through `true-bdd version` (#220).
type Stamp struct {
	Version string
	Commit  string
}

const buildIdentity = "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/buildinfo"

// BuildRelease cross-compiles the exported engine, stamped with its version and commit.
func BuildRelease(ctx context.Context, dir, binary, targetOS, targetArch string, stamp Stamp) error {
	env := git.Environment().Strip("GOFLAGS").Set(
		"GOOS="+targetOS, "GOARCH="+targetArch, "CGO_ENABLED=0", "GOWORK=off")
	ldflags := "-ldflags=-X " + buildIdentity + ".version=" + stamp.Version +
		" -X " + buildIdentity + ".commit=" + stamp.Commit

	result, err := shell.Run(ctx, []string{Bin, buildVerb, trimPathFlag, ldflags, "-o", binary, enginePackage},
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

// DropHarness removes a copied fixture's local requirement and replacement of the test library,
// so the host requires it the way a user's project does.
func DropHarness(ctx context.Context, opt shell.Options) error {
	result, err := shell.Run(ctx, []string{Bin, modVerb, "edit",
		"-droprequire=" + harnessModule, "-dropreplace=" + harnessModule}, opt)
	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return fmt.Errorf("dropping local harness: %s %s: %w", result.Stdout, result.Stderr, err)
	}

	return nil
}
