package architecture

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// ErrLibraryDownload stops work before a writer receives an incomplete library graph.
var ErrLibraryDownload = errors.New("library download failed")

// DownloadLibraries prepares the declared graph and returns only its source and version metadata roots.
func (a *Architecture) DownloadLibraries(ctx context.Context, host string) ([]string, error) {
	libraries := a.libraries()
	if len(libraries) == 0 {
		return nil, nil
	}

	directory := filepath.Join(host, "tmp/library-download")

	_, err := disk.DirWithin(host, "tmp/library-download", disk.Private)
	if err != nil {
		return nil, fmt.Errorf("prepare library download: %w", err)
	}

	directory, err = disk.TempDir(directory, "graph-")
	if err != nil {
		return nil, fmt.Errorf("prepare isolated library graph: %w", err)
	}
	defer func() { _ = disk.RemoveTree(directory) }()

	var manifest strings.Builder
	manifest.WriteString("module true-bdd-library-download\n\ngo 1.25.0\n\nrequire (\n")

	names := make([]string, 0, len(libraries))
	for _, library := range libraries {
		name := library.Module + "@" + library.Version
		if slices.Contains(names, name) {
			continue
		}

		names = append(names, name)

		fmt.Fprintf(&manifest, "%s %s\n", library.Module, library.Version)
	}

	manifest.WriteString(")\n")

	err = disk.Write(filepath.Join(directory, "go.mod"), []byte(manifest.String()), disk.Private)
	if err != nil {
		return nil, fmt.Errorf("prepare libraries %s: %w", names, err)
	}

	modules, err := gotool.DownloadModules(ctx, directory)
	if err != nil {
		return nil, fmt.Errorf("%w (%s): %w", ErrLibraryDownload, strings.Join(names, ", "), err)
	}

	roots, err := libraryReadRoots(modules, libraries)
	if err != nil {
		return nil, err
	}

	slog.Info("Libraries downloaded", "libraries", names, "read_roots", roots)

	return roots, nil
}

func libraryReadRoots(modules []gotool.Module, libraries []Library) ([]string, error) {
	roots := []string{}
	versions := make(map[string]string)

	for _, module := range modules {
		if module.Dir == "" || module.GoMod == "" || module.Info == "" || module.Zip == "" {
			return nil, fmt.Errorf("%w: incomplete cache for %s@%s: %s", ErrLibraryDownload,
				module.Path, module.Version, module.Error)
		}

		versions[module.Path] = module.Version
		roots = append(roots, module.Dir, module.GoMod, module.Info, module.Zip, module.Zip+"hash")
	}

	for _, library := range libraries {
		if versions[library.Module] != library.Version {
			return nil, fmt.Errorf("%s@%s resolved to %s: %w", library.Module, library.Version,
				versions[library.Module], ErrInvalidLibrary)
		}
	}

	slices.Sort(roots)

	return slices.Compact(roots), nil
}
