package architecture

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

// CodeWritableRoots protects declared test directories as well as Registry suites.
func (a *Architecture) CodeWritableRoots(host string, suites []string) ([]WritableRoot, error) {
	host, err := filepath.EvalSymlinks(host)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}

	tests := append([]string(nil), suites...)

	base := host
	if a.Testing.ConfigFile != "" {
		base = filepath.Dir(a.Testing.ConfigFile)
		if !filepath.IsAbs(base) {
			base = filepath.Join(host, base)
		}

		resolved, resolveErr := resolveWritablePath(base, 0)
		if resolveErr != nil {
			return nil, resolveErr
		}

		if withinWritable(host, resolved) {
			tests = append(tests, relativeTestPath(host, resolved))
		}
	}

	commands, err := a.commandTestDirectories(host, base)
	if err != nil {
		return nil, err
	}

	return WritableRoots(host, a.Services, append(tests, commands...))
}

func (a *Architecture) commandTestDirectories(host, base string) ([]string, error) {
	var tests []string
	if a.Testing.Framework != "go-test" {
		return tests, nil
	}

	for _, command := range []string{a.Testing.Commands.Record, a.Testing.Commands.Replay, a.Testing.Commands.Live} {
		args, err := testrunner.SplitCommand(command)
		if err != nil {
			return nil, fmt.Errorf("declared test command: %w", err)
		}

		for _, arg := range args {
			if !strings.HasPrefix(arg, "./") {
				continue
			}

			directory, err := resolveWritablePath(filepath.Join(base, strings.TrimSuffix(arg, "/...")), 0)
			if err != nil {
				return nil, err
			}

			if directory == host || !withinWritable(host, directory) || a.isServiceDirectory(host, directory) {
				continue
			}

			tests = append(tests, relativeTestPath(host, directory))
		}
	}

	return tests, nil
}

func relativeTestPath(host, directory string) string {
	relative, _ := filepath.Rel(host, directory)

	return relative
}

func (a *Architecture) isServiceDirectory(host, directory string) bool {
	for _, service := range a.Services {
		root, err := resolveWritablePath(filepath.Join(host, service.Path), 0)
		if err == nil && withinWritable(root, directory) {
			return true
		}
	}

	return false
}
