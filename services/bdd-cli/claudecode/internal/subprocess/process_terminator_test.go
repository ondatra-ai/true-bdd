package subprocess_test

import (
	"testing"
)

func TestProcessTermination(t *testing.T) {
	// Process termination needs a real running process, so it's covered by
	// integration tests rather than here.
	t.Skip("Process termination is tested in integration tests")
}
