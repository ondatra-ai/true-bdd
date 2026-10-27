package gotool

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// RestoreOffline tidies a newly seeded manifest and builds using only the prepared module cache.
func RestoreOffline(ctx context.Context, dir, binary, cache string) (string, error) {
	var transcript strings.Builder

	for _, args := range [][]string{
		{Bin, modVerb, "tidy"},
		{Bin, buildVerb, trimPathFlag, "-o", binary, "."},
	} {
		result, err := shell.Run(ctx, args, shell.Options{Dir: dir, Group: true,
			Env: git.Environment().Strip("GOFLAGS").Set("GOWORK=off", "GOTOOLCHAIN=local",
				"GOPROXY=off", "GOSUMDB=off", "GOMODCACHE="+cache,
				"GOCACHE="+filepath.Join(filepath.Dir(cache), "offline-build-cache")),
		})
		fmt.Fprintln(&transcript, strings.Join(args, " "), result.Stdout, result.Stderr)

		if err == nil {
			err = result.Err()
		}

		if err != nil {
			return transcript.String(), fmt.Errorf("offline manifest recreation: %s: %w", transcript.String(), err)
		}
	}

	return transcript.String(), nil
}
