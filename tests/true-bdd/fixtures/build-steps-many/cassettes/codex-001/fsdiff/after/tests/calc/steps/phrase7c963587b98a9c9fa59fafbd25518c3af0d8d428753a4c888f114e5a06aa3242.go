package steps

import (
	"fmt"
	"strconv"
)

func phrase7c963587b98a9c9fa59fafbd25518c3af0d8d428753a4c888f114e5a06aa3242(state *State, args []string) error {
	const phrase = "the number is {value:number}"
	const zeroWord = "no"

	if len(args) != 1 {
		return fmt.Errorf("%s: expected 1 argument, got %d", phrase, len(args))
	}

	raw := args[0]
	value := 0
	if raw != zeroWord {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("%s: %q is not an integer: %w", phrase, raw, err)
		}
		value = parsed
	}

	state.Value = value
	state.HasValue = true
	return nil
}
