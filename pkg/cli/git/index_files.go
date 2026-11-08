package git

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// IndexFiles lists tracked paths from the hook's pending index, including additions and excluding removals.
func IndexFiles() ([]string, error) {
	opt := defaults()
	if index, exists := os.LookupEnv("GIT_INDEX_FILE"); exists {
		opt.Env = opt.Env.Set("GIT_INDEX_FILE=" + index)
	}

	result, err := shell.Run(context.Background(), []string{Bin, "ls-files", "--cached", "-z"}, opt)
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("git index files: %w", result.Err())
	}

	return strings.Split(strings.TrimSuffix(result.Stdout, "\x00"), "\x00"), nil
}
