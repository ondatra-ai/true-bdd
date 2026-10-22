// Package crush is the fallback writer CLI for delegated turns.
package crush

import (
	"context"
	"io"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Options supplies a workspace, separate output streams and a watchdog.
type Options struct {
	Dir      string
	Output   io.Writer
	Report   io.Writer
	Watchdog shell.Watchdog
}

// WriteCode uses the configured model, disabling server mode to keep tools in the process group.
func WriteCode(ctx context.Context, brief string, opts Options) (shell.Result, error) {
	return shell.Watch(ctx, []string{"crush", "--cwd", opts.Dir, "run", "--quiet", "--reasoning-effort", "high"},
		shell.Options{Dir: opts.Dir, Stdin: strings.NewReader(brief),
			Env:    shell.Inherit().Set("CRUSH_CLIENT_SERVER=false"),
			Output: shell.Streams(io.MultiWriter(opts.Output, opts.Report), opts.Output)}, opts.Watchdog)
}
