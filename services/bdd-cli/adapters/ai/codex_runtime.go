package ai

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/cli/codex"
)

func codexRuntimeGrants(config *strings.Builder, env map[string]string) error {
	runtimes := codex.RuntimeExecutables()
	granted := make(map[string]bool)

	for _, name := range slices.Sorted(maps.Keys(runtimes)) {
		paths, err := codexRuntimePaths(env, name, runtimes[name])
		if err != nil {
			return err
		}

		for _, path := range paths {
			if !granted[path] {
				fmt.Fprintf(config, "%s = \"read\"\n", strconv.Quote(path))
				granted[path] = true
			}
		}
	}

	env["PATH"] += string(filepath.ListSeparator) + filepath.Join(env["HOME"], "runtime-bin")
	env["npm_config_cache"] = filepath.Join(env["HOME"], "npm-cache")
	env["npm_config_offline"] = "true"
	env["npm_config_userconfig"] = filepath.Join(env["HOME"], ".npmrc")
	env["DOCKER_CONFIG"] = filepath.Join(env["HOME"], ".docker")
	env["OPENSSL_CONF"] = "/dev/null"

	return nil
}

func codexRuntimeLink(env map[string]string, name, target string) error {
	directory := filepath.Join(env["HOME"], "runtime-bin")

	info, statErr := os.Lstat(directory)
	if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("runtime directory is a symlink: %w", os.ErrPermission)
	}

	err := disk.Dir(directory, disk.Private)
	if err != nil {
		return fmt.Errorf("create runtime directory: %w", err)
	}

	path := filepath.Join(directory, name)

	err = os.Symlink(target, path)
	if errors.Is(err, os.ErrExist) {
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr == nil && resolved == target {
			return nil
		}
	}

	if err != nil {
		return fmt.Errorf("link codex runtime %s: %w", name, err)
	}

	return nil
}

func codexRuntimeSymlinks(path string) []string {
	var links []string

	for parent := filepath.Dir(path); parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			links = append(links, parent)
		}
	}

	return links
}

func codexRuntimePaths(env map[string]string, name, executable string) ([]string, error) {
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, fmt.Errorf("resolve codex runtime %s: %w", name, err)
	}

	paths, err := codexRuntimeLibraries(resolved)
	if err != nil {
		return nil, err
	}

	err = codexRuntimeLink(env, name, resolved)
	if err != nil {
		return nil, err
	}

	if name == "npm" && filepath.Base(resolved) == "npm-cli.js" {
		paths = append(paths, filepath.Dir(filepath.Dir(resolved)))
	}

	for _, path := range slices.Clone(paths) {
		paths = append(paths, codexRuntimeSymlinks(path)...)
		if strings.HasPrefix(path, "/opt/homebrew/opt/") {
			paths = append(paths, "/opt/homebrew/opt")
		}

		if strings.HasPrefix(path, "/usr/local/opt/") {
			paths = append(paths, "/usr/local/opt")
		}

		if strings.HasSuffix(path, ".dylib") {
			paths = append(paths, filepath.Dir(path))
		}
	}

	return paths, nil
}
