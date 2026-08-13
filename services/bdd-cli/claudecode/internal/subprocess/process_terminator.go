package subprocess

import (
	"fmt"
	"os/exec"
)

// terminateProcess kills the process and waits for completion.
// Extracted to reduce duplication in timeout and cancellation handling.
func terminateProcess(cmd *exec.Cmd, done chan error, reason string) error {
	killErr := cmd.Process.Kill()
	if killErr != nil && !isProcessAlreadyFinishedError(killErr) {
		return fmt.Errorf("kill process after %s: %w", reason, killErr)
	}

	<-done

	return nil
}
