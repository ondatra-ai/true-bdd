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

// Run lints and writes the findings to out, dropping golangci's chatter.
// Captured rather than streamed: golangci prints at the end either way.
func Run(out io.Writer, args ...string) (shell.Result, error) {
	result, err := shell.Run(context.Background(), append([]string{Bin}, args...),
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
