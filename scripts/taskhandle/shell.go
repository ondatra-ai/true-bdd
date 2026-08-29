package taskhandle

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// sh runs an argv list and returns its stdout. No shell, ever, and no dief:
// commit's and merge's own helpers stop the process on failure, which is the
// one behaviour this package must not inherit.
func sh(argv ...string) (string, error) {
	//nolint:gosec // every argv in this package is a literal or a parsed value.
	command := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	// Blanked, not removed: a child should know it is not interactive. Only a
	// nested `claude -p` needs the variable gone — see claudecli.
	command.Env = append(os.Environ(), "CLAUDECODE=")

	var stdout, stderr bytes.Buffer

	command.Stdout, command.Stderr = &stdout, &stderr

	err := command.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("%s: %w: %s",
			strings.Join(argv, " "), err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

// line runs a command whose whole answer is one line.
func line(argv ...string) (string, error) {
	out, err := sh(argv...)

	return strings.TrimSpace(out), err
}

func itoa(value int) string { return strconv.Itoa(value) }
