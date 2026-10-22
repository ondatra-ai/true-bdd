package codexrun

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/ps"
)

const markerKey = "TRUE_BDD_CODEX_RUN"

func (r Runner) marker(record Record) string {
	return fmt.Sprintf("%s:%d:%x", record.Name, record.Attempt, sha256.Sum256([]byte(r.Root)))
}

func (r Runner) requireIdle(ctx context.Context, record Record) error {
	if !record.writer() {
		return nil
	}

	records, err := r.records()
	if err != nil {
		return err
	}

	records = append(records, record)

	var markers []string

	for _, prior := range records {
		if !prior.writer() || prior.Worktree != record.Worktree {
			continue
		}

		for attempt := 1; attempt <= prior.Attempt; attempt++ {
			previous := prior
			previous.Attempt = attempt
			markers = append(markers, r.marker(previous))
		}
	}

	processes, err := ps.Marked(ctx, markerKey, markers)
	if err != nil {
		return refuse(err.Error())
	}

	if len(processes) == 0 {
		return nil
	}

	var holders []string
	for _, process := range processes {
		holders = append(holders, fmt.Sprintf("%s (PID %d, marker %s)", process.Command, process.PID, process.Marker))
	}

	return refuse("marked processes still hold the worktree: " + strings.Join(holders, "; "))
}

func (r Runner) stopMarked(record Record) error {
	if !record.writer() {
		return nil
	}

	const timeout = 30 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout+r.Grace)
	defer cancel()

	err := ps.StopMarked(ctx, markerKey, r.marker(record), r.Grace)
	if err != nil {
		return fmt.Errorf("stopping processes marked %s: %w", r.marker(record), err)
	}

	return nil
}
