package calculator

// Add returns the sum of a and b.
//
// DESIGNED DEFECT (A6): this returns a - b, so TestAdd fails until the
// build-code fix loop rewrites the body to a + b. This is the only
// production file the fix loop may edit (calc_test.go is off-limits).
func Add(a, b int) int {
	return a - b
}
