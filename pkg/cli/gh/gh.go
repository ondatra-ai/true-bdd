// Package gh reads GitHub issues before a sandboxed reader starts.
package gh

import (
	"context"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// IssueJSON downloads the body, labels and comments using GitHub's wire format.
func IssueJSON(ctx context.Context, dir, number string, watch shell.Watchdog) ([]byte, error) {
	args := []string{"gh", "issue", "view", number, "--json", "number,title,state,labels,body,comments"}

	result, err := shell.Watch(ctx, args,
		shell.Options{Dir: dir}, watch)
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("reading issue #%s: %w", number, result.Err())
	}

	return []byte(result.Stdout), nil
}
