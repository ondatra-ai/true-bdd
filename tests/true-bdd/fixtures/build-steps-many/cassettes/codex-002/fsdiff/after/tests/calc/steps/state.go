package steps

// State carries what one calc scenario hands from step to step.
//
// Value is the integer argument the next `double` command runs with;
// HasValue records that a Given actually supplied it, so a When can
// refuse to run the command on an unset slot instead of silently
// defaulting to 0.
//
// Result is the integer the `double` command printed on stdout, and
// HasResult records that the command actually ran, so a Then observes
// a real run instead of comparing against a zero value.
type State struct {
	Value     int
	HasValue  bool
	Result    int
	HasResult bool
}
