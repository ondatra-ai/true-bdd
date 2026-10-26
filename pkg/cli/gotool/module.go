package gotool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Module is one downloaded Go module and its exact cache locations.
type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Dir     string `json:"dir"`
	Info    string `json:"info"`
	GoMod   string `json:"gomod"`
	Zip     string `json:"zip"`
	Error   string `json:"error"`
}

// DownloadModules downloads the complete graph of an isolated manifest, without changing the host's module files.
func DownloadModules(ctx context.Context, dir string) ([]Module, error) {
	result, err := shell.Run(ctx, []string{Bin, "mod", "download", "-json", "all"}, shell.Options{
		Dir: dir, Group: true, Env: git.Environment().Strip("GOFLAGS").Set("GOWORK=off", "GOTOOLCHAIN=local"),
	})
	if err != nil {
		return nil, fmt.Errorf("download modules: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("download modules: %s %s: %w", result.Stdout, result.Stderr, result.Err())
	}

	decoder := json.NewDecoder(strings.NewReader(result.Stdout))

	var modules []Module

	for {
		var module Module

		err = decoder.Decode(&module)
		if errors.Is(err, io.EOF) {
			return modules, nil
		}

		if err != nil {
			return nil, fmt.Errorf("decode downloaded module: %w", err)
		}

		modules = append(modules, module)
	}
}
