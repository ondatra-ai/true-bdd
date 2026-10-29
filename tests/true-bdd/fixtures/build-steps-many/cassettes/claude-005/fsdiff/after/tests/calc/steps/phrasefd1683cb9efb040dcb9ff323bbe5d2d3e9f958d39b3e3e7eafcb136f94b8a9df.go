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
	const (
		phrase   = "the number is doubled"
		wantArgs = 0
		// The suite's working directory is tests/calc, so the declared
		// "host root" of the `double` command is two levels up.
		hostRoot    = "../.."
		goBinary    = "go"
		runVerb     = "run"
		servicePath = "./services/calc"
		commandName = "double"
		runTimeout  = 2 * time.Minute
	)

	if len(args) != wantArgs {
		return fmt.Errorf("%s: want %d arguments, got %d", phrase, wantArgs, len(args))
	}

	if !state.NumberSet {
		return fmt.Errorf("%s: no number was supplied; state the number in a Given before this When", phrase)
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	// Run the declared command as a process; the product does the doubling.
	value := strconv.Itoa(state.Number)
	cmd := exec.CommandContext(ctx, goBinary, runVerb, servicePath, commandName, value)
	cmd.Dir = hostRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"%s: %s %s %s %s %s in %s failed: %w: %s",
			phrase, goBinary, runVerb, servicePath, commandName, value, hostRoot, err,
			strings.TrimSpace(stderr.String()),
		)
	}

	// The declared response is the doubled integer and a newline on stdout.
	printed := strings.TrimSpace(stdout.String())
	doubled, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("%s: stdout %q is not an integer: %w", phrase, printed, err)
	}

	// Keep the observed stdout verbatim; the Then block compares it.
	state.Result = doubled
	state.ResultSet = true

	return nil
}
