// Package calc is the fixture host project's production source. The
// build-code fix loop is allowed to edit exactly this tree.
package calc

// Add returns the sum of a and b.
func Add(a, b int) int {
	return a + b
}

// Multiply, which the suite calls, is not declared: that is the planted
// defect, in PRODUCTION source. The suite's package does not compile
// until it is.
