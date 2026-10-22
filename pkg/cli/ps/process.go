// Package ps discovers processes by inherited environment identity.
package ps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

var errOutput = errors.New("invalid ps process output")

// Process identifies a live command carrying an environment marker.
type Process struct {
	PID     int
	Command string
	Marker  string
	parent  int
	state   string
}

// Marked finds exact environment entries, excluding this process and its ancestors.
func Marked(ctx context.Context, key string, values []string) ([]Process, error) {
	if len(values) == 0 {
		return nil, nil
	}

	enriched, err := snapshot(ctx, true)
	if err != nil {
		return nil, err
	}

	plain, err := snapshot(ctx, false)
	if err != nil {
		return nil, err
	}

	ancestors := ancestry(plain)

	var found []Process

	for _, process := range enriched {
		command, present := plain[process.PID]
		if !present || ancestors[process.PID] || strings.HasPrefix(process.state, "Z") {
			continue
		}

		marker := environmentMarker(process.Command, command.Command, key, values)
		if marker != "" {
			command.Marker = marker
			command.Command = strings.Fields(command.Command)[0]
			found = append(found, command)
		}
	}

	return found, nil
}

func environmentMarker(enriched, command, key string, values []string) string {
	environment, present := strings.CutPrefix(enriched, command+" ")
	if !present {
		return ""
	}

	for _, entry := range strings.Fields(environment) {
		name, value, _ := strings.Cut(entry, "=")
		if name == key && slices.Contains(values, value) {
			return value
		}
	}

	return ""
}

// StopMarked verifies that no marked descendants remain after graceful shutdown.
func StopMarked(ctx context.Context, key, value string, grace time.Duration) error {
	for {
		processes, err := Marked(ctx, key, []string{value})
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

func snapshot(ctx context.Context, environment bool) (map[int]Process, error) {
	const timeout = 10 * time.Second

	flags := "ww"
	if environment {
		flags = "eww"
	}

	result, err := shell.Run(ctx, []string{"ps", flags, "-U", strconv.Itoa(os.Getuid()), "-o", "pid=,ppid=,stat=,args="},
		shell.Options{Dir: "/", Timeout: timeout, Group: true})
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		result.Stdout = "" // A partial environment snapshot must never become an error message.

		return nil, fmt.Errorf("discovering environment markers with ps: %w", result.Err())
	}

	return parse(result.Stdout)
}

func parse(output string) (map[int]Process, error) {
	processes := make(map[int]Process)

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)

		const columns = 4
		if len(fields) < columns {
			return nil, errOutput
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			return nil, errOutput
		}

		parent, err := strconv.Atoi(fields[1])
		if err != nil || parent < 0 {
			return nil, errOutput
		}

		rest := strings.TrimSpace(line)
		for range columns - 1 {
			rest = strings.TrimLeft(rest[len(strings.Fields(rest)[0]):], " \t")
		}

		processes[pid] = Process{PID: pid, parent: parent, state: fields[2], Command: rest}
	}

	return processes, nil
}

func ancestry(processes map[int]Process) map[int]bool {
	processes[os.Getpid()] = Process{parent: os.Getppid()}

	ancestors := make(map[int]bool)
	for pid := os.Getpid(); pid > 0 && !ancestors[pid]; pid = processes[pid].parent {
		ancestors[pid] = true
	}

	return ancestors
}
