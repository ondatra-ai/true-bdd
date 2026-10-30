package steps

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type State struct{ Result int }

type Registrar interface {
	Step(string, func(*State, []string) error)
}

// Halve runs the calculator's halve command with the whole number and
// keeps its integer stdout as the result.
func (s *State) Halve(value string) error {
	if os.Getenv("CALC_LICENCE") == "" {
		return errors.New("the calculator's licence is not installed: set CALC_LICENCE")
	}
	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	s.Result, err = strconv.Atoi(strings.TrimSpace(string(out)))
	return err
}
