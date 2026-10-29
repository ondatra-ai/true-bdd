package steps

type State struct {
	// Number is the integer argument the next `double` command run receives.
	Number int
	// NumberSet records that a Given supplied Number, so the When can
	// refuse to run `double` with an unstated operand.
	NumberSet bool
	// Result is the integer the `double` command printed on stdout, kept
	// exactly as observed so the Then block only compares.
	Result int
	// ResultSet records that the `double` command ran and its stdout was
	// read, so the Then can refuse to compare an unobserved result.
	ResultSet bool
}
