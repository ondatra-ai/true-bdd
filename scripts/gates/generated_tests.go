package gates

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/cli/truebdd"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// GeneratedTests builds the engine and checks this repository through its public command.
func GeneratedTests() error {
	root, err := git.TopLevel()
	if err != nil {
		return fmt.Errorf("locate generated-test host: %w", err)
	}

	binary := filepath.Join(root, "bin", "true-bdd-gate")

	err = disk.Dir(filepath.Dir(binary), disk.Private)
	if err != nil {
		return fmt.Errorf("prepare gate binary: %w", err)
	}

	err = gotool.Build(cli.Options{}, "services/true-bdd", binary, ".")
	if err != nil {
		return fmt.Errorf("build suite generator: %w", err)
	}

	result, err := truebdd.VerifyTests(context.Background(), binary, cli.Options{
		Dir: root, Env: git.Environment().Set("GOWORK=off"),
	})
	if err != nil {
		return fmt.Errorf("run suite generator: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("verify generated suite: %w\n%s%s", result.Err(), result.Stdout, result.Stderr)
	}

	return nil
}
