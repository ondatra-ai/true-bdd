// Package cli substitutes record/replay stand-ins for a host's CLI dependencies.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const libraryModule = "github.com/ondatra-ai/true-bdd/pkg"
const utilityPackage = "github.com/ondatra-ai/true-bdd/utilities/cliproxy"
const buildTimeout = 2 * time.Minute

var errNoRevision = errors.New("library module has no repository commit")

// Utility is a stand-in binary built from the same repository commit as the host's library.
type Utility struct{ binary string }

// Build prepares the stand-in from the host's library version, using a sibling utilities module for local library
// copies.
func Build(host, directory string) (*Utility, error) {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	source, err := gotool.ResolveModule(ctx, host, libraryModule)
	if err != nil {
		return nil, fmt.Errorf("locate proxy library: %w", err)
	}

	if source.Replace != nil {
		source = source.Replace
	}

	err = disk.Dir(directory, disk.Private)
	if err != nil {
		return nil, fmt.Errorf("create proxy directory: %w", err)
	}

	binary := filepath.Join(directory, "cliproxy")
	if source.Version == "" {
		err = buildLocal(ctx, source.Dir, directory, binary)
	} else {
		var revision string

		revision, err = moduleRevision(ctx, host, source)
		if err == nil {
			err = gotool.InstallVersion(ctx, cli.Options{Dir: host}, directory, utilityPackage+"@"+revision)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("build CLI stand-in: %w", err)
	}

	return &Utility{binary: binary}, nil
}

func buildLocal(ctx context.Context, library, directory, binary string) error {
	utilities := filepath.Join(filepath.Dir(library), "utilities")

	workspace, err := disk.TempDir(directory, "workspace-")
	if err != nil {
		return fmt.Errorf("create proxy workspace: %w", err)
	}

	defer func() { _ = disk.RemoveTree(workspace) }()

	path := filepath.Join(workspace, "go.work")

	content := "go 1.25.0\n\nuse (\n" + strconv.Quote(library) + "\n" + strconv.Quote(utilities) + "\n)\n"

	err = disk.Write(path, []byte(content), disk.Private)
	if err != nil {
		return fmt.Errorf("write proxy workspace: %w", err)
	}

	return gotool.BuildWorkspace(ctx, cli.Options{}, path, utilities, binary, "./cliproxy")
}

func moduleRevision(ctx context.Context, host string, source *gotool.ModuleSource) (string, error) {
	// Pseudo-versions already name their commit; module-specific tags do not.
	pseudo, _ := regexp.MatchString(`^v\d+\.\d+\.\d+-(?:0\.)?\d{14}-[a-f0-9]{12}$`, source.Version)
	if pseudo {
		_, revision, _ := strings.Cut(strings.TrimPrefix(source.Version, "v"), "-")
		_, revision, _ = strings.Cut(revision, "-")

		return revision, nil
	}

	resolved, err := gotool.DownloadSource(ctx, host, source.Path+"@"+source.Version)
	if err != nil {
		return "", fmt.Errorf("resolve library commit: %w", err)
	}

	if resolved.Origin == nil || resolved.Origin.Hash == "" {
		return "", errNoRevision
	}

	return resolved.Origin.Hash, nil
}

// Substitute installs the stand-in under dependency names in directory, which the caller places first on PATH.
func (u *Utility) Substitute(directory string, dependencies []Dependency) error {
	err := disk.Dir(directory, disk.Private)
	if err != nil {
		return fmt.Errorf("create substitution directory: %w", err)
	}

	rules := make(map[string]Recording, len(dependencies))
	for _, dependency := range dependencies {
		name := dependency.Name

		rules[name] = dependency.Recording
		if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
			return fmt.Errorf("%w: invalid CLI name %q", errCLIName, name)
		}

		err := os.Symlink(u.binary, filepath.Join(directory, name))
		if err != nil {
			return fmt.Errorf("substitute CLI %s: %w", name, err)
		}
	}

	data, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("encode recording rules: %w", err)
	}

	err = disk.Write(filepath.Join(directory, "recording.json"), data, disk.Private)
	if err != nil {
		return fmt.Errorf("write recording rules: %w", err)
	}

	return nil
}

var errCLIName = errors.New("CLI name must be a basename")
