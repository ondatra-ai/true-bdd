package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	if state.Program == "" {
		return fmt.Errorf("double %s: the calculator is not installed", args[0])
	}
	command := exec.Command(state.Program, "double", args[0])
	command.Dir = "../.."
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run calc double %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	doubled, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("calc double %s printed %q: %w", args[0], stdout.String(), err)
	}
	state.Value = doubled
	return nil
}
