// Package calc is the fixture host project's production source. The
// build-code fix loop is allowed to edit exactly this tree.
package calc

// Add returns the sum of a and b.
//
// It does not: the operator is wrong, deliberately. This is the planted
// defect the fix loop has to correct, and it is in PRODUCTION source —
// the test asserting 5 is right, and repairing the code rather than the
// assertion is the whole contract of `build code`.
func Add(a, b int) int {
	return a + b
}
