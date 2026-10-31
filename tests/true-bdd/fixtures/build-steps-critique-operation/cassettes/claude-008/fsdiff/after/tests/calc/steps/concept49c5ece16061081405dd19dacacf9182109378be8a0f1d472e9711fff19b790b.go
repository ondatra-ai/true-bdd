package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	const hostRoot = "../.."
	if state.Program == "" {
		return fmt.Errorf("double %s: the calculator is not installed", args[0])
	}
	command := exec.Command(state.Program, "double", args[0])
	command.Dir = hostRoot
	stdout, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("run calc double %s: %w: %s", args[0], err, exit.Stderr)
		}
		return fmt.Errorf("run calc double %s: %w", args[0], err)
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("read the doubled integer from calc double %s stdout %q: %w", args[0], stdout, err)
	}
	state.Value = value
	return nil
}
