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
	const hostRoot = "../.."
	number := args[0]
	if state.Program == "" {
		return fmt.Errorf("double %s: the calculator is not installed", number)
	}
	command := exec.Command(state.Program, "double", number)
	command.Dir = hostRoot
	var errors bytes.Buffer
	command.Stderr = &errors
	stdout, err := command.Output()
	if err != nil {
		if message := strings.TrimSpace(errors.String()); message != "" {
			return fmt.Errorf("double %s: %w: %s", number, err, message)
		}
		return fmt.Errorf("double %s: %w", number, err)
	}
	printed := strings.TrimSpace(string(stdout))
	value, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("double %s: read the doubled integer %q: %w", number, printed, err)
	}
	state.Value = value
	return nil
}
