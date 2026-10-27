package commands

import (
	"context"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
)

func (p *buildTestsPrep) prepareLibraries(ctx context.Context, arch *architecture.Architecture, walking int) error {
	if !p.fix || walking == 0 || p.libraryApplier == nil {
		return nil
	}

	roots, err := arch.DownloadLibraries(ctx, p.repoRoot)
	if err != nil {
		return err
	}

	p.libraryApplier.UseLibraryReadRoots(roots)

	return nil
}
