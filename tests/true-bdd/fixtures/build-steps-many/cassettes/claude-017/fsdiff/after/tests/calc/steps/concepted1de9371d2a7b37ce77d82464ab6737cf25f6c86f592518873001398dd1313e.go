package steps

import (
	"fmt"

	"fixture/stepsrun/tests/calc/helpers"
)

//truebdd:concept then "result"
func concepted1de9371d2a7b37ce77d82464ab6737cf25f6c86f592518873001398dd1313e(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("the result step expects the expected whole number as its one value, got %d", len(args))
	}
	// args[0] is the expected whole number as the step wrote it, optionally
	// signed. The observed number is only the integer the kept run of the
	// calculator's double command printed on stdout, so this step compares
	// and never doubles anything itself.
	return helpers.CheckDoubled(state.Doubled, args[0])
}
