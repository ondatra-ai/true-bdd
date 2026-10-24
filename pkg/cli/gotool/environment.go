package gotool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

const jsonOutputFlag = "-json"

// Environment resolves the installed Go runtime and module cache.
func Environment(ctx context.Context) (map[string]string, error) {
	result, err := shell.Run(ctx, []string{Bin, "env", jsonOutputFlag, "GOROOT", "GOMODCACHE"}, shell.Options{})
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("go env: %w", result.Err())
	}

	var paths map[string]string

	err = json.Unmarshal([]byte(result.Stdout), &paths)
	if err != nil {
		return nil, fmt.Errorf("decode go env: %w", err)
	}

	return paths, nil
}
