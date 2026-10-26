// Package npm installs the test dependencies of a rehearsal's Jest host.
package npm

import (
	"context"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// InstallTests prepares the fixture's tests/package.json without a global install.
func InstallTests(ctx context.Context, opt shell.Options) error {
	result, err := shell.Run(ctx, []string{"npm", "install", "--prefix", "tests", "--no-audit", "--no-fund"}, opt)
	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return fmt.Errorf("installing Jest: %w", err)
	}

	return nil
}
