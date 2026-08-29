package shell

import (
	"io"
	"os"
	"time"
)

// Options tune one command. The zero value inherits the environment, captures
// both streams into Result, and imposes no deadline.
type Options struct {
	// Dir is the child's working directory; empty inherits the parent's.
	Dir string
	// Env is the child's environment.
	Env Env
	// Stdin is what the child reads; nil is an immediately-closed stdin.
	Stdin io.Reader
	// Output is where the child's streams go.
	Output Sink
	// Timeout bounds the command. Zero means no bound.
	Timeout time.Duration
	// Group makes the child its own process group leader and cancels it by
	// killing the group, so a cancelled context takes its children too.
	Group bool
	// WaitDelay bounds how long Wait lingers once the context has ended.
	WaitDelay time.Duration
	// ExtraFiles are descriptors the child inherits from fd 3 upwards. Only
	// remote/managed_child.go passes any: a lock fd and a release pipe.
	ExtraFiles []*os.File
}
