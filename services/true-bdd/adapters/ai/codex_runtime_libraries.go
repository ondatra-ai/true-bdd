package ai

import (
	"bytes"
	"debug/macho"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// codexRuntimeLibraries names Mach-O dependencies exactly, without opening an installation prefix.
func codexRuntimeLibraries(executable string) ([]string, error) {
	paths := []string{executable}
	seen := make(map[string]bool)

	for offset := 0; offset < len(paths); offset++ {
		path, resolveErr := filepath.EvalSymlinks(paths[offset])
		if resolveErr != nil {
			return nil, fmt.Errorf("resolve runtime image: %w", resolveErr)
		}

		if seen[path] {
			continue
		}

		seen[path] = true

		libraries, err := codexMachLibraries(path, filepath.Dir(executable))
		if err != nil {
			return nil, err
		}

		for _, library := range libraries {
			if !seen[library] {
				paths = append(paths, library)
			}
		}
	}

	return paths, nil
}

func codexMachLibraries(path, executableDir string) ([]string, error) {
	data, err := disk.Read(path)
	if err != nil {
		return nil, fmt.Errorf("read runtime image: %w", err)
	}

	image, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		fat, fatErr := macho.NewFatFile(bytes.NewReader(data))
		if fatErr != nil {
			return codexNonMachImage(data, err)
		}

		image = fat.Arches[0].File
	}

	names, err := image.ImportedLibraries()
	if err != nil {
		return nil, fmt.Errorf("read runtime dependencies: %w", err)
	}

	var paths []string

	for _, name := range names {
		if strings.HasPrefix(name, "/usr/lib/") || strings.HasPrefix(name, "/System/") {
			continue
		}

		candidates := codexLibraryCandidates(image, name, filepath.Dir(path), executableDir)

		resolved, alias, err := codexLibraryPath(candidates)
		if err != nil {
			return nil, err
		}

		paths = append(paths, alias, resolved)
	}

	return paths, nil
}

func codexLibraryCandidates(image *macho.File, name, loaderDir, executableDir string) []string {
	expand := func(path string) string {
		path = strings.ReplaceAll(path, "@loader_path", loaderDir)

		return strings.ReplaceAll(path, "@executable_path", executableDir)
	}
	if !strings.HasPrefix(name, "@rpath/") {
		return []string{expand(name)}
	}

	var candidates []string

	for _, load := range image.Loads {
		if rpath, ok := load.(*macho.Rpath); ok {
			candidates = append(candidates, filepath.Join(expand(rpath.Path), strings.TrimPrefix(name, "@rpath/")))
		}
	}

	return candidates
}

func codexLibraryPath(candidates []string) (string, string, error) {
	for _, candidate := range candidates {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil {
			return resolved, candidate, nil
		}
	}

	return "", "", fmt.Errorf("resolve runtime library %q: %w", candidates, os.ErrNotExist)
}

func codexNonMachImage(data []byte, parseErr error) ([]string, error) {
	const magicLength = 4
	if len(data) < magicLength {
		return nil, nil
	}

	magic := string(data[:magicLength])
	if magic == "\xcf\xfa\xed\xfe" || magic == "\xfe\xed\xfa\xcf" || magic == "\xca\xfe\xba\xbe" {
		return nil, fmt.Errorf("decode runtime image: %w", parseErr)
	}

	return nil, nil
}
