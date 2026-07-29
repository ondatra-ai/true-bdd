package remote

import "errors"

// Sentinel errors for the remote package. Dynamic context is attached
// by wrapping these with fmt.Errorf("...: %w", errSentinel).
var (
	// errUnexpectedStatus is returned when a harness RPC replies with a
	// non-2xx HTTP status.
	errUnexpectedStatus = errors.New("harness server returned unexpected status")
	// errRequestTooLarge is returned when a harness RPC replies 413 (the
	// request exceeded the route's byte budget). The inventory uploader
	// treats it specially: it re-scans against a strictly smaller budget
	// rather than retrying the same oversized payload (plan §1a).
	errRequestTooLarge = errors.New("harness server rejected request as too large")
	// errUnknownCommand is returned when a dispatched run names a command
	// the remote cannot map to CLI arguments (the server allowlist should
	// have rejected it first).
	errUnknownCommand = errors.New("unknown run command")
	// errNoStoryID is returned when a story command arrives without the
	// required story id.
	errNoStoryID = errors.New("story command requires a story id")
	// errMissingExecutable is returned when the remote cannot resolve its
	// own binary path to spawn children.
	errMissingExecutable = errors.New("cannot resolve remote executable path")
)
