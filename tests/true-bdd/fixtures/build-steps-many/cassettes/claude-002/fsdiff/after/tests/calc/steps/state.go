package steps

type State struct {
	// Number is the integer argument the next `double` command run receives.
	Number int
	// NumberSet records that a Given supplied Number, so the When can
	// refuse to run `double` with an unstated operand.
	NumberSet bool
}
