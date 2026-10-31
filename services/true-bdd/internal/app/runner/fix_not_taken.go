package runner

import "errors"

// ErrFixNotTaken marks a PostFix that refused to commit what the applier
// wrote: the walk asks the Prompt again, and the refusal reaches the next
// fix as a finding of that Prompt's failed result.
var ErrFixNotTaken = errors.New("the fix was not committed")
