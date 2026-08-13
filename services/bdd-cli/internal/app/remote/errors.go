package remote

import "errors"

// Sentinel errors for the remote package. Dynamic context is attached
// by wrapping these with fmt.Errorf("...: %w", errSentinel).
var (
	// errUnexpectedStatus is returned when a harness RPC replies with a
	// non-2xx HTTP status.
	errUnexpectedStatus = errors.New("harness server returned unexpected status")
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
	// errNoChildStdin is returned when an answer arrives but the child's stdin
	// is not (yet) available — the executor has no live child to deliver to.
	errNoChildStdin = errors.New("no child stdin available for answer delivery")
	// errChatNoAnchor is returned when a deterministic chat probe directive
	// cannot find its expected schema anchor (e.g. a `terms:` key) in the
	// current file content.
	errChatNoAnchor = errors.New("chat probe directive anchor not found in current file")
	// errChatUnknownDirective is returned for an unrecognized `@probe` directive.
	errChatUnknownDirective = errors.New("unknown chat probe directive")
)
