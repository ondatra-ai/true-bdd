package steps

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	const (
		// command is the double command of the declared Calculator CLI; the
		// step's capture is its one required VALUE argument.
		command = "double"
		// wantCaptures is the pattern's single capture, the whole number.
		wantCaptures = 1
	)

	if len(args) != wantCaptures {
		return fmt.Errorf("doubling: the step gave %d captures, want %d", len(args), wantCaptures)
	}

	value := args[0]

	if state.Program == "" || state.HostRoot == "" {
		return fmt.Errorf("doubling: no calculator program is installed, so %s %s cannot be run", command, value)
	}

	// The binding runs the kept program as a process from the host root,
	// passing the step's whole number as VALUE. The inherited environment
	// carries this run's Go cache and temporary directory.
	run := exec.Command(state.Program, command, value)
	run.Dir = state.HostRoot
	run.Env = os.Environ()

	var stdout, stderr bytes.Buffer

	run.Stdout = &stdout
	run.Stderr = &stderr

	err := run.Run()
	if err != nil {
		return fmt.Errorf("doubling: %s %s %s in %s: %w\n%s",
			state.Program, command, value, state.HostRoot, err, stderr.String())
	}

	// The CLI document declares exit 0 prints the doubled integer and a
	// newline. The step only reads that integer; it never works out what
	// the double of the number should be.
	printed := strings.TrimSpace(stdout.String())

	doubled, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("doubling: %s %s %s printed %q on stdout, not the integer the CLI document declares: %w",
			state.Program, command, value, stdout.String(), err)
	}

	state.Result = doubled
	state.Doubled = true

	return nil
}
