// Package golint is the `golangci-lint` command line, one of the typed
// wrappers pkg/shell may be reached through.
//
// It owns the chatter filter as well as the spawn: golangci prints nine lines
// of exclusion-rule bookkeeping per run, which buries the finding in what the
// edit hook hands back. Knowing that is knowing this binary, so it lives here
// rather than in the gate that calls it.
package golint

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "golangci-lint"

// chatterPrefix marks the lines golangci writes about its own configuration.
const chatterPrefix = "level=warning"

// FixFlag asks golangci to rewrite what it can.
const FixFlag = "--fix"

// Lint lints the named directories — the whole module when none are named —
// rewriting what it can when fix is set, and writes the findings to out with
// golangci's chatter dropped. A non-zero Result.Code is a finding.
func Lint(out io.Writer, fix bool, dirs ...string) (shell.Result, error) {
	argv := []string{Bin, "run"}
	if fix {
		argv = append(argv, FixFlag)
	}

	result, err := shell.Run(context.Background(), append(argv, dirs...),
		shell.Options{Output: shell.Combined()})
	if err != nil {
		return result, fmt.Errorf("running %s: %w", Bin, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(result.Stdout))
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), bufio.MaxScanTokenSize)

	for scanner.Scan() {
		if !strings.HasPrefix(scanner.Text(), chatterPrefix) {
			_, _ = fmt.Fprintln(out, scanner.Text())
		}
	}

	return result, nil
}
