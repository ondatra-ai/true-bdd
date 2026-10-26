package git

import (
	"context"
	"fmt"
	"time"
)

// CommitTime returns the committer timestamp used by Go pseudo-versions.
func (r Repository) CommitTime(ctx context.Context, commit string) (time.Time, error) {
	stamp, err := r.output(ctx, "show", "-s", "--format=%cI", commit)
	if err != nil {
		return time.Time{}, err
	}

	parsed, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse commit time: %w", err)
	}

	return parsed.UTC(), nil
}
