package drill

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/ps"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Stop asks the detached worker to clean up and waits for its final record.
func (r Runner) Stop(ctx context.Context, directory, tag string) error {
	record, err := loadUnchecked(directory)
	if err != nil {
		return err
	}

	run, ok := record.Runs[tag]
	if !ok {
		return fmt.Errorf("%w: unknown run", errRefused)
	}

	if !run.EndedAt.IsZero() {
		return nil
	}

	err = disk.Write(filepath.Join(directory,
		"runs",
		tag,
		"stop"),
		[]byte("operator stop\n"),
		disk.Private)
	if err != nil {
		return err
	}

	return r.waitStopped(ctx, directory, tag, run.Marker)
}

func (r Runner) waitStopped(ctx context.Context, directory, tag, marker string) error {
	ticker := time.NewTicker(r.Poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for stopped run: %w", ctx.Err())
		case <-ticker.C:
			current, err := loadUnchecked(directory)
			if err != nil {
				return err
			}

			final := current.Runs[tag]
			if !final.EndedAt.IsZero() {
				return nil
			}

			workers, err := ps.Marked(ctx, "TRUE_BDD_DRILL_WORKER", []string{marker})
			if err != nil {
				return err
			}

			if len(workers) == 0 {
				current, err = loadUnchecked(directory)
				if err != nil {
					return err
				}

				if !current.Runs[tag].EndedAt.IsZero() {
					return nil
				}

				return r.finish(directory, tag, -1, fmt.Errorf("%w: stopped without a worker", errRefused))
			}
		}
	}
}
