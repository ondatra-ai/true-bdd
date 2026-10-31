package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	number := args[0]

	if _, err := strconv.Atoi(number); err != nil {
		return fmt.Errorf("the step's whole number %q is not an integer literal: %w", number, err)
	}

	if state.CalcPath == "" {
		return fmt.Errorf("no calculator program was kept by the installation Given; the doubling step needs it to run double %s", number)
	}

	hostRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return fmt.Errorf("locate the host root: %w", err)
	}

	run := exec.Command(state.CalcPath, "double", number)
	run.Dir = hostRoot
	run.Env = os.Environ()

	var stderr strings.Builder
	run.Stderr = &stderr

	stdout, err := run.Output()
	if err != nil {
		return fmt.Errorf("run %s double %s from %s: %w: %s", state.CalcPath, number, hostRoot, err, strings.TrimSpace(stderr.String()))
	}

	printed := strings.TrimSpace(string(stdout))
	doubled, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("%s double %s printed %q on stdout, which is not the doubled integer the Calculator CLI declares: %w", state.CalcPath, number, printed, err)
	}

	state.Doubled = doubled
	state.DoubledSet = true
	return nil
}
