# One `claude` path, and it is isolated

ADR 0005 made `pkg/cli/<tool>` the only way to reach a binary, and its `Start`
table admitted six single-site exceptions. One of them was a whole second
implementation: `services/bdd-cli/claudecode`, 43 files of vendored SDK
speaking a bidirectional `stream-json` protocol over pipes, listed as
"a bidirectional JSON protocol (`transport.go:526-529`)" because a pipe
deadlocks once the child fills it.

It complied with 0005 — it reached `os/exec` through `pkg/cli` like everything
else. That is why nothing caught what it actually was: a second answer to
"how does this repo spawn `claude`", maintained in parallel with
`pkg/cli/claude` and diverging from it in the one way that mattered.

## What the second path cost

Fixture `build-code-fix-named-test`, recorded 2026-09-01, fixing a one-line
bug (`Add(2,3)` returning `-1`): **228.6s wall, five AI turns, $3.84**. Of that
228.6s, 227.3s was inside the five turns; the Go side, including both `go test`
runs, was 1.3s.

Every one of the four `claude` turns called an `advisor` tool the engine never
configured — visible in each cassette as `{"name":"advisor","input":{}}`
followed by `advisor_message` / `advisor_tool_result`. Turn 1 spent 31.8s of
its 42.0s there. Across the four turns the re-recorded cassettes' `modelUsage`
splits the bill exactly:

| | opus-5 (the engine's work) | fable-5 (the advisor) |
|---|---|---|
| claude-001 | $0.5169 | $0.5997 |
| claude-002 | $0.7732 | $0.7425 |
| claude-003 | $0.2712 | $0.5617 |
| claude-004 | $0.2726 | $0.5587 |
| **total** | **$1.8339** | **$2.4626** |

The tool nobody asked for outspent the engine. The child also fired three
`SessionStart` hooks and carried a ~47k-token system prefix against an
engine prompt of ~1.5k — the engine's own words were about 3% of what the
model read.

## Why the tool gate did not hold

The transport already passed one:

```text
--allowed-tools     Read(**),Write(./tmp/**),Glob(**),Grep(**)
--disallowed-tools  Bash,Edit(**),MultiEdit(**),Agent,Task
```

`advisor` is in neither list and ran anyway, because it is a **server-side**
tool: `--allowed-tools` and `--disallowed-tools` gate the tools the CLI runs
locally, and a server-injected tool is not among them. Naming `advisor` in
`--disallowed-tools` would look like a fix and change nothing. The only gate
is `CLAUDE_CODE_DISABLE_ADVISOR_TOOL=1`.

The rest came in the same way. The operator's `~/.claude/settings.json` had no
`hooks` block at all — the three `SessionStart` hooks came from its four
`enabledPlugins`, and the advisor's model from its `advisorModel: "fable"`.
Both are **user-scope** settings, and the child loaded user scope because
nothing told it not to.

## The decision

Delete `services/bdd-cli/claudecode`. `ClaudeProvider.Execute` was already
documented as "runs one single-turn prompt" and sent exactly one
`client.Query` per `WithClient`, so the bidirectional protocol was answering a
question the engine never asked. What the provider needed from it — the
answer, and what the turn cost — `pkg/cli/claude` already returned:
`Answer.CostUSD` and `Answer.Tokens`, parsed from the `-p --output-format json`
envelope. `RunJSON` now forces that envelope with or without a schema, so a
turn that answers in prose still reports its own bill.

The wrapper grew a second form rather than one. `RunStream` runs the same
single prompt under `--output-format stream-json --verbose` and hands each
record to a callback as it arrives; `RunJSON` reads the one-shot envelope.
The engine takes the streaming form and `scripts/` the envelope, because the
difference is not the answer — it is whether anyone needs to know WHEN the
turn first spoke and which tools it called. `pkg/testkit/reporter` does:
`FirstOutput`, `ResultAt` and `ToolCalls` are folded from those records
(`engine_log.go:395-405`), and `ToolCalls` is the column that would have
shown the advisor. Both forms end at one `parseEnvelope`.

Isolation is not a knob on the wrapper. Every turn carries
`--setting-sources project --strict-mcp-config` and
`CLAUDE_CODE_DISABLE_ADVISOR_TOOL=1`, because a caller able to opt back into
the operator's environment is precisely the bug: the leak was never a decision
anyone made, it was the default nobody overrode. `project` is kept rather than
cut to nothing — a host workspace's own `.claude/settings.json` carries the
permissions the turn is meant to run under, and denying those would deny the
turn the writes its prompt instructs it to make.

## Why not `--bare`

`--bare` buys the same isolation in one flag and two things this engine cannot
pay. It skips CLAUDE.md auto-discovery, and a host workspace's `CLAUDE.md` is
designed input — `build-code-fix-named-test`'s says "the failing assertion is
the specification, and the production code is what is wrong", which is the
rule the fix loop exists to obey. And it restricts auth to `ANTHROPIC_API_KEY`
or `apiKeyHelper`, never reading OAuth or the keychain, so an operator who
signed in interactively would spawn turns that cannot authenticate at all.

## What this does not do

It does not touch the `crush` or `codex` paths, which were never duplicated.

It does not claim the isolation is free of behaviour change. Cutting user
scope changes what every `claude` turn reads, so **every cassette recorded
before this is stale** — argv changed too, from `--output-format stream-json
--verbose --input-format stream-json` to the `-p` envelope. Replay exits 86
until the fixtures are re-recorded; that is the expected state of this change,
not a regression to debug.

It does not keep every DEBUG line the SDK emitted. `Processing content block`
and the message-type narration are gone; what survives is exactly what the
report folds on, and `MsgToolUse` moved into `pkg/enginelog` so the producer
and the reporter share one constant instead of two literals drifting apart.

It does change which model `scripts/` turns run. Ten call sites across
`scripts/commit`, `scripts/merge` and `scripts/clickup` never set
`Options.Model`, so they had been inheriting the operator's user-scope
`model` — the very scope this ADR cuts. That choice is now configuration:
`scripts.models` in `true-bdd/true-bdd.yaml`, resolved command, then script,
then default. The block sits in the host's config file because it is the
config file this repo has; the engine reads `engine:` and ignores it.

**Amends ADR 0005.** Its `Start` table loses the `transport.go:526-529` row;
five single-site callers remain. `.golangci.yaml` still names the deleted
package in `hex-domain` (line 195) and in an exclusion path (line 326). Both
are inert — a deny for a package that cannot be imported, and a path that
matches nothing — and are left for a change that has permission to edit that
file.
