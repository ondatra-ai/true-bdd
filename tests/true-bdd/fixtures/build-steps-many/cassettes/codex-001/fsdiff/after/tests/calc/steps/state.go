package steps

type State struct {
	// Number is the integer argument the next `double` command run receives,
	// stored by the `the number is {value:number}` Given.
	Number int
	// NumberSet reports whether a Given has supplied Number yet.
	NumberSet bool
}
