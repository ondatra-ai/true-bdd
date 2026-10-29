package steps

import (
	"fmt"
	"strconv"
)

func phrase7c963587b98a9c9fa59fafbd25518c3af0d8d428753a4c888f114e5a06aa3242(state *State, args []string) error {
	const (
		phrase    = "the number is {value:number}"
		zeroWord  = "no"
		wantArgs  = 1
		valueSlot = 0
	)

	if len(args) != wantArgs {
		return fmt.Errorf("%s: want %d argument, got %d", phrase, wantArgs, len(args))
	}

	raw := args[valueSlot]

	value := 0
	if raw != zeroWord {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("%s: value %q is not a number: %w", phrase, raw, err)
		}
		value = parsed
	}

	// Store the operand as the next `double` command argument; the When
	// block runs the command and the Then block observes its stdout.
	state.Number = value
	state.NumberSet = true

	return nil
}
