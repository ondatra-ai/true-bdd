# `start.sh` is the only shell script

`.alint.yml`'s `no-shell` rule refuses every `.sh` and `.bash` file in the
repository except `start.sh`, plus the two fenced trees every other rule there
already excludes. Sixteen scripts — about 1,200 lines — were ported to Go under
`scripts/` to satisfy it.

## Why the tooling could not stay in shell

This repository's tooling lives under `scripts/` rather than `.claude/` because
the Go tool skips any directory whose name begins with a dot: a package under
`.claude/` is invisible to `go build ./...`, `go test ./...` and golangci-lint.
The shell was the unfinished half of that argument. It was invisible to all
three wherever it sat, and the thin `.sh` shims under `.claude/` that used to
`go run` each command were the seam where that invisibility survived.

What that cost is not hypothetical. `diff-context.sh` exists because a 3.45 MB
diff piped to `claude -p` answered "Prompt is too long" into a redirect, so
pull request #70 produced no commit message and no visible error (ClickUp
86cb6g6q8). The scripts it was extracted from carry four separate comments
about SIGPIPE interacting with `set -o pipefail` — `head -c` on a pipe, `sed`
after `head`, a command substitution returning 141 — each one a bug found in
production and paid for with a comment rather than a test.

The comment-budget gate was the clearest case: 114 lines of shell wrapping an
awk state machine, gating every comment in the repository, with no test of its
own. It is now `scripts/lint`, with fourteen. The port was verified by running
both implementations over the whole tree and diffing the findings; they were
byte-identical before the shell was deleted.

Two dependencies disappeared with the shell rather than being ported: `jq`
(the hook's payload parsing) and `curl` (the CLAUDE.md mirror fetch). `yamale`,
`markdownlint-cli2` and `alint` remain, because the Go code execs them.

## Why `start.sh` is exempt

It exports `.env` before `claude` starts. Nothing launched *inside* a session
can do that — a key sourced mid-session never reaches the subprocesses that
need it — so this one file has to be the shell that runs before there is a Go
process to run instead. It is 21 lines and calls `exec`.

## What is fenced rather than exempt

`tests/bdd-cli/fixtures/*/prep.sh` and `teardown.sh` are not tooling: they are
designed host-project content, named by scenarios as Given steps and read by
the harness *by content* (`.claude/rules/bdd-harness.md`). Banning them would
change the engine's contract with its hosts, which is a different decision from
this one. `tests/legacy/` is fenced for the same reason every other rule fences
it — it exists to be deleted.

The two skills whose *product* is a bash script keep it. `wizard` generates an
interactive walkthrough for a human, and `diagnosing-bugs` a human-in-the-loop
repro harness; both templates were renamed to `*.sh.tmpl`, which the rule does
not match, and generated wizards are written into gitignored `./tmp/`, which
alint does not see. The rule is about what this repository maintains, not about
what its skills emit for someone else to run.

## The cost accepted

A `go run` is slower to start than a `bash` shim, and the PostToolUse lint hook
pays that on every edit. It is a compile of already-cached packages, tens of
milliseconds against the gate's own runtime, and the hook's budget is 120s.

Renaming the two templates also drops them out of `.alint.yml`'s
final-newline, trailing-whitespace and line-ending rules, and out of the
comment gate. That is accepted: they are emitted artifacts, not maintained
tooling, and `wizard/SKILL.md` already forbids hand-editing the library half.
