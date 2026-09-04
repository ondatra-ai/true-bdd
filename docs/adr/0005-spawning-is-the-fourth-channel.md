# Spawning is the fourth channel, and `pkg/cli` is how it is reached

ADR 0003 admitted `pkg/` for three IO channels and said each is "the single
mechanism for one kind of IO, and `forbidigo` refuses every other way of doing
it." Spawning a subprocess was the kind of IO it did not name. `os/exec` was
imported by 45 files across all three roots and owned by nobody.

`pkg/shell` is now the only place `os/exec` is imported, and `pkg/cli/<tool>`
the only place `pkg/shell` is called from.

## Why the omission cost something

Three packages carried the same helper. `scripts/taskhandle/shell.go:16`,
`scripts/merge/exec.go:48` and `scripts/commit/run.go:192` each defined `sh()`
over `bytes.Buffer` pairs, with the same `//nolint:gosec` and the same
`CLAUDECODE=` comment copied verbatim between them. Exit-code extraction was
written four times (`merge/exec.go:153`, `claudecli/run.go:184`,
`testrunner/spawn_log.go:144`, `remote/terminal_envelope.go:114`), `LookPath`
preflight eleven, and the `Setpgid` + `Cancel` + `WaitDelay` block twice
verbatim (`adapters/ai/cli_invocation.go:46-60`,
`infrastructure/stepcoverage/reader.go:271-280`).

Duplication was the symptom. The defect was that the three copies disagreed
about failure and only their comments recorded it: merge's stops the run twice
over (on timeout, and on `check`), commit's `sh` never stops but its
`gitChecked` does, and taskhandle's says in its own doc comment that not
stopping "is the one behaviour this package must not inherit." Three policies,
one name, no signature that distinguished them.

So `shell` terminates nothing. A non-zero exit is `Result.Code`; `Run`'s error
means the command did not run to completion at all. Every stop is now written
at the call site that wants it.

## Why a second package sits above it

A ban with nowhere to go produces an exemption list. `pkg/console` is denied to
`scripts/**` with five negations today, each a judgment call re-litigated by
hand, and that list only grows.

`pkg/cli/<tool>` is the somewhere to go. Each package owns one binary's argv:
`git`, `github`, `claude`, `gotool`, `crush`, `codex`, plus `spec` for argv a
host config supplies and `bash` for the fixture-authored strings that
`tests/libraries/runner` runs. The rule a developer meets is not "you may not
spawn" but "spawn through the wrapper, and write one if it is missing" — which
is what the linter's message says.

The layering is proved rather than trusted, the same way ADR 0003 argued for
`pkg/logging` getting no `disk` or `console` exemption: `pkg/cli` is exempt
from the `pkg/shell` deny **only**, so it still cannot import `os/exec`. The
compiler and depguard together make "cli reaches subprocesses through shell"
a checked fact.

`_test.go` is exempt from the `os/exec` deny, following `scripts-console` and
err113. Several tests exist precisely to synthesise what `shell` abstracts:
`terminal_envelope_internal_test.go:17,:36` needs a real `*exec.ExitError` and
a `Signaled()` wait status, `main_internal_test.go:96` deliberately orphans a
`Start()` with no `Wait()`.

## Blank is not strip, and the difference reaches the model

`Env` distinguishes them because the tree already did, in opposite directions,
and a shared helper is exactly where that would have been flattened.

Three sites append `CLAUDECODE=`, blanking it. `scripts/merge/exec.go:60-62`
says why: "a child should know it is not interactive." Three others remove the
key outright — `claudecli/run.go:71-73` is explicit that this is not the same
thing:

> not blanked like other subprocess helpers do: a nested `claude -p` must look
> entirely unlaunched-from-a-session.

`tests/libraries/runner/runner.go:501` strips for the same reason. Collapsing
blank and strip into one "clear this variable" option would change what the
agent CLIs believe about their own launch, silently, with every cassette still
replaying green until a live run disagreed.

## Two tiers, and why the second is not a general API

`Run` covers roughly 35 sites: spawn, wait, read. `Start` exists for five that
cannot be expressed that way, and it was tempting to leave those on raw
`os/exec` with a documented exemption. That was rejected for the reason above —
an exemption list is a ban that stops being checkable.

What `Start` carries is therefore single-site and stays that way; each option
names its one caller where it is declared:

| capability | sole site | why |
|---|---|---|
| `ExtraFiles` | `remote/managed_child.go:76,115` | lock fd, release pipe on fd 3 |
| signal forwarding | `aiproxy/record.go:201` | relays SIGTERM/SIGINT to the proxied CLI |
| stdin from the console | `remote/supervisor.go:55` | raw descriptor passthrough |

The rule for adding to that table is the rule for the exemption list it
replaced: a second caller, or a comment naming the first.

## What this does not do

It does not forbid a shell interpreter. `.alint.yml`'s `no-shell` bans shell
*files*, never `bash -c` argv, and three production sites run fixture-authored
command strings where the string is the contract. Enforcing "argv only, no
interpreter" would be a further rule and is deliberately not taken here.

**Amended 2026-08-30.** An earlier amendment on this date moved `bash`, `cp` and
`ps` into `pkg/shell` as `BashRun`, `CpRecursive` and `PsOutput`, on the ground
that their wrappers "held one argv literal and nothing else". That measured the
wrong thing. The knowledge had not gone anywhere — it had moved to the callers:
`supervisor.go` grew twenty lines of process-table parsing and a field-count
constant, `harness.go` encoded `cp`'s copy-the-contents idiom as the string
`+ "/."`, and the three `bash -c` loops diverged, the materializer's losing the
deadline the runner's had. Judge a wrapper by what its callers hold.

So the three are gone from `pkg/shell` and `pkg/cli` both, and what replaced them
is deeper than what was deleted: `pkg/cli/ps` answers `GroupMembers` and
`StartedAt` with the column parsing inside; `pkg/cli/spec.Phase` runs a fixture's
whole `prep:`/`teardown:` list under one shared budget, which is where the three
loops' divergence went; and `cp -R` is not spawned at all, because `pkg/disk`
owns tree copy (it refuses a symlink rather than flattening one silently). ADR
0003's claim holds again as written: `pkg/shell` is the only importer of
`os/exec`, and `pkg/cli/<tool>` the only caller of `pkg/shell`, with no named-file
exemption list in `.golangci.yaml` or `.alint.yml`.

The same measurement condemns a wrapper that only *proxies*. `git.Run(args...)`,
`github.Output(args...)` and `gotool.Output(args...)` left every verb, `--json`
field list and `--jq` selector at the call site, so `scripts/commit` and
`scripts/merge` each grew a private `sh`/`git`/`gh` layer over `pkg/cli/spec` —
the three `sh()` copies this ADR claimed to have removed, back within one
release. The generic entry points are therefore unexported. What a caller names
is an operation: `github.SquashMerge`, `git.HasStagedChanges`,
`markdownlint.Lint`, `yamale.Validate`. What stays at the call site is what was
always the caller's — the report spans, the stop policy, the poll budgets and
the prompts.

It also does not touch `pkg/logging` or `pkg/console`. Removing the child
descriptors drops roughly eight importers from `console` as a side effect, and
that is as far as it goes: `logging.Install`'s `Stream` parameter, whose doc
still names a stdout parser that no longer exists, is left for its own change.

**Amended 2026-09-04.** The `Start` table's `stderr to a real file` row named
`subprocess/transport.go:526-529`, the vendored SDK that ran the engine's
`claude` turns. That package is deleted (ADR 0010): it complied with this ADR
while being a second answer to the question this ADR exists to have one answer
to, and the divergence was not academic — the SDK path spawned children that
inherited the operator's user-scope settings, and one fixture paid $2.46 of a
$4.30 bill to an advisor tool nobody configured. `Run`'s two-tier claim is
unchanged; there are five single-site `Start` callers now, not six.
