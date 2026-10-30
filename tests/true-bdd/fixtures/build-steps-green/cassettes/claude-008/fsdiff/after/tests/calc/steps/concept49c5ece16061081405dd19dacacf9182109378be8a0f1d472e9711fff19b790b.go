package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	const hostRoot = "../.."
	const commandPath = "./services/calc"
	const commandName = "double"

	value := strings.TrimSpace(args[0])
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("the Owner doubles %s: not a whole number: %w", args[0], err)
	}

	cmd := exec.Command("go", "run", commandPath, commandName, value)
	cmd.Dir = hostRoot
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go run %s %s %s in %s: %w: %s", commandPath, commandName, value, hostRoot, err, strings.TrimSpace(stderr.String()))
	}

	printed := strings.TrimSpace(stdout.String())
	doubled, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("go run %s %s %s: stdout %q is not an integer: %w", commandPath, commandName, value, printed, err)
	}

	state.Program = commandPath
	state.Value = doubled
	return nil
}
