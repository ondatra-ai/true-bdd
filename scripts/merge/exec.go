package merge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// diagnosticLimit is how much of a failed command's output a stop quotes.
const diagnosticLimit = 800

// A porcelain record is `XY <path>`: two status columns and a space.
const statusPrefix = 3

// labelWords is how much of an argv a diagnostic names. Enough to identify
// the command, short enough not to paste a prompt into an error message.
const labelWords = 4

// options tune one command. The zero value captures output, imposes no
// deadline, and lets a non-zero exit through to the caller.
type options struct {
	// timeout bounds the command. Zero means no bound.
	timeout time.Duration
	// check stops the run on a non-zero exit.
	check bool
	// stream sends output to the terminal instead of capturing it, for the
	// long commands whose progress is the point.
	stream bool
}

// result is one finished command.
type result struct {
	stdout string
	stderr string
	code   int
}

// sh runs an argv list. No shell, ever.
func (r *Run) sh(cmd []string, opt options) result {
	ctx := context.Background()

	if opt.timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, opt.timeout)
		defer cancel()
	}

	//nolint:gosec // every argv in this package is a literal or a parsed API value.
	command := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	// Blanked, not removed: a child should know it is not interactive. Only a
	// nested `claude -p` needs the variable gone entirely — see claudecli.
	command.Env = append(os.Environ(), "CLAUDECODE=")

	var stdout, stderr bytes.Buffer

	if opt.stream {
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
	} else {
		command.Stdout, command.Stderr = &stdout, &stderr
	}

	err := command.Run()

	if ctx.Err() != nil {
		r.dief("%s timed out after %s", label(cmd), opt.timeout)
	}

	finished := result{stdout: stdout.String(), stderr: stderr.String(), code: exitCode(err)}

	if opt.check && finished.code != 0 {
		r.dief("%s failed (%d):\n%s", label(cmd), finished.code,
			textutil.Truncate(firstNonEmpty(finished.stderr, finished.stdout), diagnosticLimit))
	}

	return finished
}

// git runs a git command against the checkout.
func (r *Run) git(args ...string) result {
	return r.sh(append([]string{gitBin}, args...), options{})
}

// gitChecked runs a git command and stops the run if it fails.
func (r *Run) gitChecked(args ...string) string {
	return r.sh(append([]string{gitBin}, args...), options{check: true}).stdout
}

// gh runs a gh command and stops the run if it fails.
func (r *Run) gh(args ...string) string {
	return r.sh(append([]string{ghBin}, args...), options{check: true}).stdout
}

// ghJSON runs a gh command and decodes its answer.
func (r *Run) ghJSON(target any, args ...string) {
	out := strings.TrimSpace(r.gh(args...))
	if out == "" {
		return
	}

	err := json.Unmarshal([]byte(out), target)
	if err != nil {
		r.dief("could not read the answer to `gh %s`:\n%s",
			strings.Join(args, " "), textutil.Truncate(out, diagnosticLimit))
	}
}

// worktreeChanges is what git reports as uncommitted — empty when clean.
// Returns the raw listing for callers that print it; changedPaths answers
// the same question as a set.
func (r *Run) worktreeChanges() string {
	return strings.TrimSpace(r.gitChecked("status", "--porcelain"))
}

// changedPaths is the set of paths git reports as uncommitted. -z: the
// default format quotes any path with a space or non-ASCII byte, and a
// quoted name matches nothing it's compared against.
func (r *Run) changedPaths() map[string]bool {
	paths := map[string]bool{}
	entries := strings.Split(r.gitChecked("status", "--porcelain", "-z"), "\x00")

	for index := 0; index < len(entries); index++ {
		entry := entries[index]
		if entry == "" {
			continue
		}

		if entry[0] == 'R' {
			index++ // a rename emits its source as a second record
		}

		if len(entry) > statusPrefix {
			paths[filepath.Clean(entry[statusPrefix:])] = true
		}
	}

	return paths
}

func label(cmd []string) string {
	return strings.Join(cmd[:min(labelWords, len(cmd))], " ")
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return -1
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
