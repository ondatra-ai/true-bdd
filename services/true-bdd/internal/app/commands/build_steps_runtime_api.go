package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/gotool"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// prepareRuntimeAPI saves the host-resolved public meanings once, before any body turn.
func (e *buildStepsExecution) prepareRuntimeAPI(ctx context.Context, arch *architecture.Architecture) error {
	if arch.Testing.Testkit == nil || e.runtimeAPI != nil {
		return nil
	}

	host, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("locate runtime API host: %w", err)
	}

	e.runtimeAPI = make(map[string]string)

	for _, name := range []string{"testkit", "proxy/cli"} {
		imported := "github.com/ondatra-ai/true-bdd/pkg/" + name

		documentation, docErr := gotool.PackageDocumentation(ctx, host, imported)
		if docErr != nil {
			return docErr
		}

		path := filepath.Join(e.deps.TmpDir, "runtime-api", filepath.FromSlash(name)+".txt")
		if !filepath.IsAbs(path) {
			path = filepath.Join(host, path)
		}

		err = disk.Write(path, []byte(documentation), disk.Private)
		if err != nil {
			return fmt.Errorf("save Go documentation for %s: %w", imported, err)
		}

		e.runtimeAPI[imported] = path
	}

	return nil
}
