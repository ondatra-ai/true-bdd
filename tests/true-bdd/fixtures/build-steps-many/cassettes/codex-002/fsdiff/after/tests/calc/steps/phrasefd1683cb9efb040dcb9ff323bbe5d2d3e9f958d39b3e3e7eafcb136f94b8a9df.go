package steps

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func phrasefd1683cb9efb040dcb9ff323bbe5d2d3e9f958d39b3e3e7eafcb136f94b8a9df(state *State, args []string) error {
	const phrase = "the number is doubled"
	// The suite's package directory is tests/calc, so the host root the
	// declared CLI document runs `double` from is two levels up.
	const hostRoot = "../.."
	const command = "double"
	const runTimeout = 2 * time.Minute

	if len(args) != 0 {
		return fmt.Errorf("%s: expected 0 arguments, got %d", phrase, len(args))
	}
	if !state.HasValue {
		return fmt.Errorf("%s: no number supplied; a Given must set the number first", phrase)
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	value := strconv.Itoa(state.Value)
	cmd := exec.CommandContext(ctx, "go", "run", "./services/calc", command, value)
	cmd.Dir = hostRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"%s: running %s %s from %s failed: %w: %s",
			phrase, command, value, hostRoot, err, strings.TrimSpace(stderr.String()),
		)
	}

	raw := strings.TrimSpace(stdout.String())
	result, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("%s: stdout %q is not an integer: %w", phrase, raw, err)
	}

	state.Result = result
	state.HasResult = true
	return nil
}
