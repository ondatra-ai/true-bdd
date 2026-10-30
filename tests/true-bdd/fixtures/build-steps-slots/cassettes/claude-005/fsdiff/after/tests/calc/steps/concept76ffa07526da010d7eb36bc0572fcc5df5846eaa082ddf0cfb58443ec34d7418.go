package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "running"
func concept76ffa07526da010d7eb36bc0572fcc5df5846eaa082ddf0cfb58443ec34d7418(state *State, args []string) error {
	if state.Program == "" {
		return fmt.Errorf("the calculator is not installed")
	}
	command := exec.Command(state.Program, args[0], args[1])
	command.Dir = "../.."
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("run the calculator %q on %s: %w", args[0], args[1], err)
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return fmt.Errorf("read the calculator stdout %q: %w", string(output), err)
	}
	state.Value = value
	return nil
}
