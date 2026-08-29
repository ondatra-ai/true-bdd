// Package shell is every subprocess this repository spawns, and the only
// place os/exec is imported.
//
// It is the fourth channel. pkg/console owns terminal bytes, pkg/disk owns
// files, pkg/logging owns records; spawning was the one kind of IO nobody
// owned, and it showed: scripts/taskhandle, scripts/merge and scripts/commit
// each carried a copy of the same 25-line sh(), down to identical comments,
// and the three disagreed about whether a failure stops the process. Exit-code
// extraction was written four times, LookPath eleven.
//
// Nothing here terminates the process. A non-zero exit is Result.Code, not an
// error — Run's error means the command did not run to completion at all
// (never started, or the context expired). Callers that want a stop write it
// themselves: that policy differed silently between the three predecessors and
// is now stated at each call site.
//
// TWO TIERS. Run covers the ~35 sites that spawn, wait and read. Start returns
// a Process for the six that cannot: a bidirectional JSON protocol
// (services/bdd-cli/claudecode/.../transport.go), supervised process groups
// holding inherited descriptors (internal/app/remote/managed_child.go), a
// byte-exact stdio proxy (tests/libraries/aiproxy), and a long-lived server
// (tests/bdd-web/steps/harness.go). Their needs are single-site and each is
// documented where it is declared.
//
// THE ENV DISTINCTION IS LOAD-BEARING. Blank sets a key to empty; Strip
// removes it. Three sites blank CLAUDECODE, because a child should know it is
// not interactive. Three remove it, because a nested `claude -p` must look
// entirely unlaunched-from-a-session. Collapsing the two changes what the
// agent CLIs do.
//
// Callers reach a binary through pkg/cli/<tool>, never through here directly:
// depguard denies pkg/shell everywhere except pkg/cli/**, which is what keeps
// argv construction typed and greppable instead of scattered.
package shell
