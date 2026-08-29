package subprocess

import (
	"fmt"
	"os"

	spawn "github.com/ondatra-ai/true-bdd/pkg/cli"
)

// terminateProcess kills the process and waits for completion.
// Extracted to reduce duplication in timeout and cancellation handling.
func terminateProcess(proc *spawn.Process, done chan error, reason string) error {
	killErr := proc.Signal(os.Kill)
	if killErr != nil && !isProcessAlreadyFinishedError(killErr) {
		return fmt.Errorf("kill process after %s: %w", reason, killErr)
	}

	<-done

	return nil
}
