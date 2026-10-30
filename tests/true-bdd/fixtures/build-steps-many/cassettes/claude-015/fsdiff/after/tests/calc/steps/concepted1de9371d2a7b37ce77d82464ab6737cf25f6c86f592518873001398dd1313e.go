package steps

import (
	"fmt"
	"strconv"
	"strings"
)

//truebdd:concept then "result"
func concepted1de9371d2a7b37ce77d82464ab6737cf25f6c86f592518873001398dd1313e(state *State, args []string) error {
	const commandName = "double"

	expectedText := args[0]

	expected, err := strconv.Atoi(expectedText)
	if err != nil {
		return fmt.Errorf(
			"read the expected whole number %q written in the step: %w",
			expectedText, err,
		)
	}

	if state.DoubleStdout == "" {
		return fmt.Errorf(
			"observe the whole number the %q command printed, want %d: no stdout was kept from the %q command",
			commandName, expected, commandName,
		)
	}

	printed := strings.TrimSpace(state.DoubleStdout)

	actual, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf(
			"read a whole number from the stdout %q the %q command printed: %w",
			state.DoubleStdout, commandName, err,
		)
	}

	if actual != expected {
		return fmt.Errorf(
			"the %q command printed %d on stdout, want the result %d",
			commandName, actual, expected,
		)
	}

	return nil
}
