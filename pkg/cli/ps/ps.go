// Package ps is the `ps` command line, one of the typed wrappers pkg/shell
// may be reached through.
//
// Both questions this repository asks the process table are answered here
// rather than by a caller reading columns: `-o` selects fields whose layout
// is ps's, not ours, and the two callers were each parsing that layout for
// themselves. What a caller names is a process group or a pid.
package ps

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "ps"

// groupFields is what `-o pgid=,pid=` prints per row: the trailing `=` on each
// field suppresses the header, so every readable line is exactly two columns.
const groupFields = 2

// GroupMembers is every pid in the process group, in the order ps lists them.
// An empty slice means the group is empty, which is not an error; a table that
// could not be read is.
func GroupMembers(pgid int) ([]int, error) {
	out, err := output("-A", "-o", "pgid=,pid=")
	if err != nil {
		return nil, err
	}

	var members []int

	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != groupFields {
			continue
		}

		group, groupErr := strconv.Atoi(fields[0])
		pid, pidErr := strconv.Atoi(fields[1])

		if groupErr != nil || pidErr != nil {
			continue
		}

		if group == pgid {
			members = append(members, pid)
		}
	}

	return members, nil
}

// StartedAt is the process's start time as ps prints `lstart=`: stable for its
// whole life and different across a recycled pid, so it is an identity rather
// than a clock. Empty when the pid is gone — ps exits non-zero for one.
func StartedAt(pid int) (string, error) {
	result, err := run("-o", "lstart=", "-p", strconv.Itoa(pid))
	if err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", nil
	}

	return strings.TrimSpace(result.Stdout), nil
}

// run spawns ps. A non-zero exit is Result.Code: an unknown pid is one, and
// the caller above reads it as an answer rather than a failure.
func run(args ...string) (shell.Result, error) {
	result, err := shell.Run(context.Background(), append([]string{Bin}, args...),
		shell.Options{})
	if err != nil {
		return result, fmt.Errorf("running %s: %w", Bin, err)
	}

	return result, nil
}

// output is run for the queries whose non-zero exit is a failure, returning
// stdout UNTRIMMED because GroupMembers splits it into lines.
func output(args ...string) (string, error) {
	result, err := run(args...)
	if err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("%s %s: %w", Bin, strings.Join(args, " "), result.Err())
	}

	return result.Stdout, nil
}
