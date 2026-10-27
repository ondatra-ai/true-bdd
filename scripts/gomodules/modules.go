// Package gomodules declares the repository's Go modules, excluding fixture sentinels.
package gomodules

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
)

const allPackages = "./..."

// Directories are the three modules checked locally and in CI.
func Directories() []string { return []string{".", "pkg", "services/true-bdd"} }

// Owner resolves a directory to its repository module; sentinel modules select nothing.
func Owner(dir string) (string, string, bool) {
	dir = filepath.Clean(dir)
	for walk := dir; ; walk = filepath.Dir(walk) {
		if slices.Contains(Directories(), walk) {
			relative, err := filepath.Rel(walk, dir)

			return walk, relative, err == nil
		}

		_, err := os.Stat(filepath.Join(walk, "go.mod"))
		if err == nil {
			return "", "", false
		}

		if walk == "." || filepath.Dir(walk) == walk {
			return "", "", false
		}
	}
}

// Build compile-checks every module without writing binaries into source directories.
func Build() error { return check([]string{"go", "build", allPackages}) }

// Test runs each module's unit tests independently of a workspace.
func Test() error { return check([]string{"go", "test", allPackages}) }

// Vet includes the BDD sources and generated tests in every module.
func Vet() error { return check([]string{"go", "vet", "-tags", "bdd", allPackages}) }

func check(command []string) error {
	for _, dir := range Directories() {
		result, err := spec.Run(command, cli.Options{
			Dir: dir, Env: git.Environment().Set("GOWORK=off"), Output: cli.Console(),
		})
		if err == nil {
			err = result.Err()
		}

		if err != nil {
			return fmt.Errorf("module %s: %w", dir, err)
		}
	}

	return nil
}
