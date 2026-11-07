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
	value := args[0]
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("double %q: not a whole number: %w", value, err)
	}
	if state.Program == "" {
		return fmt.Errorf("double %s: the calculator is not installed", value)
	}
	command := exec.Command(state.Program, "double", value)
	command.Dir = "../.."
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run calc double %s: %w: %s", value, err, strings.TrimSpace(stderr.String()))
	}
	doubled, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("calc double %s printed %q: %w", value, stdout.String(), err)
	}
	state.Value = doubled
	return nil
}
