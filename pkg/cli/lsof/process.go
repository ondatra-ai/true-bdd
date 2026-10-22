// Package lsof discovers and stops processes holding a working directory.
package lsof

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

var errOutput = errors.New("invalid lsof working-directory output")

// Process identifies a command holding a current working directory.
type Process struct {
	PID       int
	Command   string
	Directory string
	parent    int
}

// WorkingDirectory includes subdirectories, excluding this runner and its ancestors.
func WorkingDirectory(ctx context.Context, dir string) ([]Process, error) {
	const timeout = 10 * time.Second

	result, err := shell.Run(ctx, []string{"lsof", "-nP", "+c", "0", "-a", "-d", "cwd", "-FpcRn0"},
		shell.Options{Dir: "/", Timeout: timeout, Group: true})
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("discovering working directories with lsof: %w", result.Err())
	}

	processes, err := parse(result.Stdout)
	if err != nil {
		return nil, err
	}

	ancestors := ancestry(processes)

	var found []Process

	for _, process := range processes {
		if ancestors[process.PID] {
			continue
		}

		relative, err := filepath.Rel(dir, process.Directory)
		if err == nil && (relative == "." || filepath.IsLocal(relative)) {
			found = append(found, process)
		}
	}

	return found, nil
}

// StopWorkingDirectory verifies that no holders remain after graceful shutdown.
func StopWorkingDirectory(ctx context.Context, dir string, grace time.Duration) error {
	for {
		processes, err := WorkingDirectory(ctx, dir)
		if err != nil || len(processes) == 0 {
			return err
		}

		pids := make([]int, 0, len(processes))
		for _, process := range processes {
			pids = append(pids, process.PID)
		}

		err = shell.StopProcesses(pids, grace)
		if err != nil {
			return err
		}
	}
}

func ancestry(processes []Process) map[int]bool {
	parents := make(map[int]int, len(processes))
	for _, process := range processes {
		parents[process.PID] = process.parent
	}

	parents[os.Getpid()] = os.Getppid()

	ancestors := make(map[int]bool)
	for pid := os.Getpid(); pid > 0 && !ancestors[pid]; pid = parents[pid] {
		ancestors[pid] = true
	}

	return ancestors
}

func parse(output string) ([]Process, error) {
	if !strings.Contains(output, "\x00") {
		return nil, errOutput
	}

	var processes []Process

	current := Process{}

	for _, field := range strings.Split(output, "\x00") {
		field = strings.TrimPrefix(field, "\n")
		if field == "" {
			continue
		}

		complete, err := current.readField(field)
		if err != nil {
			return nil, err
		}

		if complete {
			processes = append(processes, current)
		}
	}

	if len(processes) == 0 {
		return nil, errOutput
	}

	return processes, nil
}

func (p *Process) readField(field string) (bool, error) {
	switch field[0] {
	case 'p', 'R':
		return false, p.readID(field)
	case 'c':
		p.Command = field[1:]
	case 'f':
		return false, nil
	case 'n':
		p.Directory = field[1:]
		if !p.valid() {
			return false, errOutput
		}

		return true, nil
	default:
		return false, errOutput
	}

	return false, nil
}

func (p *Process) readID(field string) error {
	pid, err := strconv.Atoi(field[1:])
	if err != nil {
		return fmt.Errorf("reading lsof process ID: %w", err)
	}

	if field[0] == 'p' {
		*p = Process{PID: pid}
	} else {
		p.parent = pid
	}

	return nil
}

func (p *Process) valid() bool {
	return p.PID > 0 && p.Command != "" && filepath.IsAbs(p.Directory)
}
