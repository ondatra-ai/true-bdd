# `pkg/cli` is what we spawn; what spawns us gets its own root

ADR 0005 gave every subprocess this repository *starts* a home: `pkg/shell` owns
`os/exec`, `pkg/cli/<tool>` owns one binary's argv. It said nothing about the other
direction, because at the time there was one program started *by* something else and
it lived where it was used.

`pkg/` now takes inbound protocols as roots of their own, one per runner:
`pkg/claude/hooks` is the PostToolUse event Claude Code hands a hook and the verdict
it reads back, and `pkg/alint` is the `kind: command` child alint runs per matched
file. Each exports one entry point taking a closure; a command under `scripts/cmd/`
supplies the closure and nothing else.

## Why not under `pkg/cli`

Both had an obvious home there — `pkg/cli/claude` and `pkg/cli/alint` already exist —
and taking it is wrong twice.

`pkg/cli/<tool>` means one thing precisely: the argv this repository hands a binary.
It means it because `depguard`'s `spawn-shell` list makes `pkg/cli` the only importer
of `pkg/shell`, which is what turns "cli reaches subprocesses through shell" into a
checked fact rather than a convention. A package under that tree which spawns nothing
weakens the claim the tree exists to make — the exemption stops describing the
directory.

And the two directions share no vocabulary. `pkg/cli/claude` knows `-p`,
`--allowedTools` and `--output-format`, pinned by `TestArgsOrder` because 14 callers
depend on the order; `pkg/claude/hooks` knows `tool_input.file_path` and that `reason`
is discarded unless `decision` is `"block"`. `pkg/cli/alint` knows `--format json` and
`TRUEBDD_SCOPE`; `pkg/alint` knows `ALINT_PATH` and that `--fix` arrives in the argv.
Neither half of either pair constrains the other. One package holding both would be
two glossaries sharing a name.

## Why not `scripts/`

Because that is where the first one was, and the cost is on the record.
`scripts/lint.Hook` parsed the payload, made the path repo-relative, decided what was
judgeable, ran the gates and encoded the verdict — five concerns in one function, of
which two are the protocol and three are the lint gate. A second hook would have
copied the two, which is exactly the `sh()` duplication ADR 0005 recorded: three
copies of one mechanism, disagreeing in ways only their comments knew.

`pkg/` is the floor every root may import. A protocol two roots could need belongs on
the floor even while one root needs it.

## The line, and what it costs

Direction decides, and nothing else: **who started the process.** It is a fact about
a program, not a judgment about coupling, which is what makes it hold under pressure.

An earlier draft of this decision drew the line at coupling instead — co-locate the
two directions when one decision spans them, since `TRUEBDD_SCOPE`, the manifest and
`--fix` really are one decision seen from two sides. That was rejected. A coupling
test is re-argued at every new package and answers differently depending on who is
arguing; a direction test is read off the code. The coupling is real and survives as
prose: `pkg/cli/alint` and `pkg/alint` each name the other in their package comments,
which is what a cross-reference is for.

What this costs is two packages called `alint`. No file needs both — the hook command
takes the outbound half, the gate command the inbound one — and an import alias
settles it if one ever does.

## Consequences

`pkg-is-the-declared-channels` in `.alint.yml` now excludes `pkg/claude/**` and
`pkg/alint/**`. That allowlist is the decision record — a package under `pkg/` not on
it fails the build — so this ADR is what those two entries point at.

Both roots will grow along their own protocol and not otherwise. PreToolUse,
SessionStart and Stop are the same Claude Code contract read at other moments: each is
a file beside `post_tool_use.go`, not a new decision. What needs a new one is a package
under either root that is not a protocol its runner speaks.
