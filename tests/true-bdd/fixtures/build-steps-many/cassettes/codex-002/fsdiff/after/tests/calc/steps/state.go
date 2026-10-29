package steps

type State struct {
	// Number is the integer argument the next `double` command run receives,
	// stored by the `the number is {value:number}` Given.
	Number int
	// NumberSet reports whether a Given has supplied Number yet.
	NumberSet bool
	// Result is the integer stdout of the last `double` command run, kept by
	// the `the number is doubled` When for the Then to compare.
	Result int
	// ResultSet reports whether a When has run `double` and recorded Result.
	ResultSet bool
}
