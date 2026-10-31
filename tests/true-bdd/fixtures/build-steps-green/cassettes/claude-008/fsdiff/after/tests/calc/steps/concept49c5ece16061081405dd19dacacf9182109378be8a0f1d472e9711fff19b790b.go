package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	const (
		hostRoot    = "../.."
		servicePath = "./services/calc"
		command     = "double"
	)

	value := args[0]

	run := exec.Command("go", "run", servicePath, command, value)
	run.Dir = hostRoot
	var stderr strings.Builder
	run.Stderr = &stderr

	stdout, err := run.Output()
	if err != nil {
		return fmt.Errorf("go run %s %s %s in %s: %w: %s",
			servicePath, command, value, hostRoot, err, strings.TrimSpace(stderr.String()))
	}

	printed := strings.TrimSpace(string(stdout))
	doubled, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("%s %s printed %q, want an integer on stdout: %w",
			command, value, printed, err)
	}

	state.Value = doubled
	return nil
}
