package codexrun

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/lsof"
)

func (r Runner) requireIdle(ctx context.Context, record Record) error {
	processes, err := lsof.WorkingDirectory(ctx, record.Workdir)
	if err != nil {
		return refuse(err.Error())
	}

	if len(processes) == 0 {
		return nil
	}

	var holders []string
	for _, process := range processes {
		holders = append(holders, fmt.Sprintf("%s (PID %d, cwd %s)", process.Command, process.PID, process.Directory))
	}

	return refuse("processes still hold the working directory: " + strings.Join(holders, "; "))
}

func (r Runner) stopWorkdir(record Record) error {
	const timeout = 30 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout+r.Grace)
	defer cancel()

	err := lsof.StopWorkingDirectory(ctx, record.Workdir, r.Grace)
	if err != nil {
		return fmt.Errorf("stopping processes in %s: %w", record.Workdir, err)
	}

	return nil
}
