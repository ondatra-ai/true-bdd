package steps

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func phrasefd1683cb9efb040dcb9ff323bbe5d2d3e9f958d39b3e3e7eafcb136f94b8a9df(state *State, args []string) error {
	// When: run the declared `double` command as a process and keep its
	// integer stdout. Per docs/architecture/calc-cli.yaml the command is
	// `go run ./services/calc double VALUE`, run from the host root, and its
	// stdout is the doubled integer plus a newline. No arithmetic happens
	// here — the product computes the result, this step only observes it.
	const phrase = "the number is doubled"

	if len(args) != 0 {
		return fmt.Errorf("%s: expected 0 captured arguments, got %d", phrase, len(args))
	}
	if !state.NumberSet {
		return fmt.Errorf("%s: no number supplied; a Given must state the number first", phrase)
	}

	workdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%s: working directory: %w", phrase, err)
	}

	// The host root is the directory holding go.mod, which is where the
	// declared `go run ./services/calc ...` invocation is rooted.
	hostRoot := ""
	for probe := workdir; ; {
		if _, statErr := os.Stat(filepath.Join(probe, "go.mod")); statErr == nil {
			hostRoot = probe
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			break
		}
		probe = parent
	}
	if hostRoot == "" {
		return fmt.Errorf("%s: no host root (a directory with go.mod) found at or above %s", phrase, workdir)
	}

	value := strconv.Itoa(state.Number)
	cmd := exec.Command("go", "run", "./services/calc", "double", value)
	cmd.Dir = hostRoot
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if runErr := cmd.Run(); runErr != nil {
		return fmt.Errorf(
			"%s: `go run ./services/calc double %s` in %s failed: %w (stderr: %s)",
			phrase, value, hostRoot, runErr, strings.TrimSpace(stderr.String()),
		)
	}

	printed := strings.TrimSpace(stdout.String())
	result, convErr := strconv.Atoi(printed)
	if convErr != nil {
		return fmt.Errorf(
			"%s: `go run ./services/calc double %s` printed %q, not an integer: %w",
			phrase, value, printed, convErr,
		)
	}

	state.Result = result
	state.ResultSet = true

	return nil
}
