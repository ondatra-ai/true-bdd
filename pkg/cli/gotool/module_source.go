package gotool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// ModuleSource is the version and effective location selected by a host's module graph.
type ModuleSource struct {
	Path    string        `json:"path"`
	Version string        `json:"version"`
	Dir     string        `json:"dir"`
	Replace *ModuleSource `json:"replace"`
	Origin  *ModuleOrigin `json:"origin"`
}

// ModuleOrigin identifies the repository commit behind a downloaded module.
type ModuleOrigin struct {
	Hash string `json:"hash"`
}

// ResolveModule locates the exact module a host builds with, including replacements.
func ResolveModule(ctx context.Context, host, module string) (*ModuleSource, error) {
	return moduleSource(ctx, host, []string{"list", "-m", jsonOutputFlag, module})
}

// DownloadSource resolves a version to its repository origin without changing the host module.
func DownloadSource(ctx context.Context, host, module string) (*ModuleSource, error) {
	return moduleSource(ctx, host, []string{modVerb, "download", jsonOutputFlag, module})
}

func moduleSource(ctx context.Context, host string, args []string) (*ModuleSource, error) {
	result, err := shell.Run(ctx, append([]string{Bin}, args...), shell.Options{
		Dir: host, Env: git.Environment().Set("GOWORK=off"),
	})
	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return nil, fmt.Errorf("resolve module source: %s: %w", result.Stderr, err)
	}

	var source ModuleSource

	err = json.Unmarshal([]byte(result.Stdout), &source)
	if err != nil {
		return nil, fmt.Errorf("decode module source: %w", err)
	}

	return &source, nil
}

// BuildWorkspace builds a command using an isolated workspace containing its local dependencies.
func BuildWorkspace(ctx context.Context, opt shell.Options, workspace, dir, binary, pkg string) error {
	opt.Dir = dir
	opt.Env = git.Environment().Set("GOWORK=" + workspace)

	result, err := shell.Run(ctx, []string{Bin, buildVerb, "-o", binary, pkg}, opt)
	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return fmt.Errorf("build workspace command: %s: %w", result.Stderr, err)
	}

	return nil
}

// InstallVersion installs one command at an exact version into directory, without editing the host.
func InstallVersion(ctx context.Context, opt shell.Options, directory, pkg string) error {
	opt.Env = git.Environment().Set("GOWORK=off", "GOBIN="+directory)

	result, err := shell.Run(ctx, []string{Bin, "install", pkg}, opt)
	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return fmt.Errorf("install command version: %s: %w", result.Stderr, err)
	}

	return nil
}
