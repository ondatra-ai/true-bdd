package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func conceptec8b881314b929394828fe6c21c7db729a95bc1a93ad8abf8ec578b93a9e83f2(state *State, args []string) error {
	const (
		hostRoot    = "../.."
		servicePath = "./services/calc"
		commandName = "halve"
	)
	number := args[0]
	cmd := exec.Command("go", "run", servicePath, commandName, number)
	cmd.Dir = hostRoot
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", commandName, number, err, strings.TrimSpace(stderr.String()))
	}
	printed := strings.TrimSpace(stdout.String())
	halved, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("%s %s: stdout %q is not an integer: %w", commandName, number, printed, err)
	}
	state.Result = halved
	return nil
}
