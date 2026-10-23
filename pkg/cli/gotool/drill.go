package gotool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

const buildVerb = "build"

// BuildTrimmed builds an engine without embedding its source directory.
func BuildTrimmed(ctx context.Context, dir, binary string) error {
	result,
		err := shell.Run(ctx,
		[]string{Bin,
			buildVerb,
			"-trimpath",
			"-o",
			binary,
			"./services/bdd-cli"},
		shell.Options{Dir: dir,
			Group: true})
	if err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("building trimmed engine: %w", result.Err())
	}

	return nil
}

// ToolchainRoots returns the Go installation, module cache and build cache.
func ToolchainRoots(ctx context.Context) ([]string, error) {
	result,
		err := shell.Run(ctx,
		[]string{Bin,
			"env",
			"-json",
			"GOROOT",
			"GOMODCACHE",
			"GOCACHE"},
		shell.Options{Group: true})
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("go env: %w", result.Err())
	}

	roots := map[string]string{}

	err = json.Unmarshal([]byte(result.Stdout), &roots)
	if err != nil {
		return nil, fmt.Errorf("decoding go roots: %w", err)
	}

	return []string{roots["GOROOT"], roots["GOMODCACHE"], roots["GOCACHE"]}, nil
}
