package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept51ed02fffcd311c254e473f1ef5f70e51d990908173bb819dd3c0508bb93ef9b(state *State, args []string) error {
	// The host root holds ./services/calc; the generated test package runs in
	// tests/calc, the same place scenarios.go reads ../../docs/scenarios.yaml from.
	const hostRoot = "../.."

	number := args[0]

	cmd := exec.Command("go", "run", "./services/calc", "halve", number)
	cmd.Dir = hostRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return fmt.Errorf("halve %s: %w", number, err)
		}
		return fmt.Errorf("halve %s: %w: %s", number, err, detail)
	}

	printed := strings.TrimSpace(stdout.String())
	halved, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("halve %s printed %q on stdout, want a whole number: %w", number, printed, err)
	}

	state.Result = halved
	return nil
}
