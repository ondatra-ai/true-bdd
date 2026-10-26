package gates

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/scripts/release"
)

// ReleaseRehearsal builds HEAD and installs it outside the checkout without model calls.
func ReleaseRehearsal() error {
	root, err := git.TopLevel()
	if err != nil {
		return fmt.Errorf("locating rehearsal repository: %w", err)
	}

	scratch := filepath.Join(root, "tmp")

	err = disk.Dir(scratch, disk.Private)
	if err != nil {
		return fmt.Errorf("preparing rehearsal scratch: %w", err)
	}

	artifacts, err := disk.TempDir(scratch, "release-gate-*")
	if err != nil {
		return fmt.Errorf("preparing rehearsal artifacts: %w", err)
	}

	ctx := context.Background()

	err = release.Build(ctx, root, "0.0.0-rehearsal", artifacts)
	if err != nil {
		return fmt.Errorf("release build: %w", err)
	}

	_, err = release.Rehearse(ctx, root, artifacts, release.Options{})
	if err != nil {
		return fmt.Errorf("release rehearsal: %w", err)
	}

	return nil
}
