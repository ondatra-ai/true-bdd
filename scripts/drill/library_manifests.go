package drill

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

type libraryDeclaration struct {
	Module  string `yaml:"module"`
	Version string `yaml:"version"`
}

type libraryCatalog struct {
	Architecture struct {
		Testing struct {
			Libraries []libraryDeclaration `yaml:"libraries"`
		} `yaml:"testing"`
		Services []struct {
			Path      string               `yaml:"path"`
			Libraries []libraryDeclaration `yaml:"libraries"`
		} `yaml:"services"`
	} `yaml:"architecture"`
}

func libraryManifests(source, engine string) ([]byte, []byte, error) {
	original, err := disk.Read(filepath.Join(engine, goModFile))
	if err != nil {
		return nil, nil, err
	}

	module := strings.Fields(string(original))
	if len(module) < 2 || module[0] != "module" {
		return nil, nil, fmt.Errorf("%w: engine module identity missing", errRefused)
	}

	data, err := disk.Read(filepath.Join(source, "docs/architecture/architecture.yaml"))
	if err != nil {
		return nil, nil, err
	}

	var catalog libraryCatalog

	err = yaml.Unmarshal(data, &catalog)
	if err != nil {
		return nil, nil, fmt.Errorf("read architecture libraries: %w", err)
	}

	engineLibraries, all, err := catalog.libraries()
	if err != nil {
		return nil, nil, err
	}

	seed, err := pinnedManifest(module[1], engineLibraries)
	if err != nil {
		return nil, nil, err
	}

	downloads, err := pinnedManifest("true-bdd-drill-download", all)

	return seed, downloads, err
}

func (c libraryCatalog) libraries() ([]libraryDeclaration, []libraryDeclaration, error) {
	var engine []libraryDeclaration

	all := append([]libraryDeclaration(nil), c.Architecture.Testing.Libraries...)
	found := false

	for _, service := range c.Architecture.Services {
		all = append(all, service.Libraries...)
		if service.Path == "services/true-bdd" {
			found = true
			engine = service.Libraries
		}
	}

	if !found {
		return nil, nil, fmt.Errorf("%w: architecture does not declare engine", errRefused)
	}

	return engine, all, nil
}

func pinnedManifest(module string, libraries []libraryDeclaration) ([]byte, error) {
	var manifest strings.Builder
	fmt.Fprintf(&manifest, "module %s\n\ngo 1.25.0\n\nrequire (\n", module)

	seen := map[string]string{}
	for _, library := range libraries {
		if previous, exists := seen[library.Module]; exists {
			if previous != library.Version {
				return nil, fmt.Errorf("%w: conflicting versions for %s", errRefused, library.Module)
			}

			continue
		}

		seen[library.Module] = library.Version
		fmt.Fprintf(&manifest, "\t%s %s\n", library.Module, library.Version)
	}

	manifest.WriteString(")\n")

	return []byte(manifest.String()), nil
}
