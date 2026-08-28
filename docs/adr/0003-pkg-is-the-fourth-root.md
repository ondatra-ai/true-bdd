# `pkg/` is the fourth root, and holds three IO channels

`.alint.yml`'s roots rule admitted a fourth directory: `pkg/`, holding
`pkg/console`, `pkg/logging` and `pkg/disk`. Each is the single mechanism for
one kind of IO, and `forbidigo` refuses every other way of doing it.

## Why three roots could not hold three channels

The terminal channel already existed, as
`services/bdd-cli/internal/pkg/console`. Go's `internal/` rule confines it to
`services/bdd-cli/`, so `scripts/` and `tests/` could not import it and did not
try: they grew an `out io.Writer` parameter convention and 240 raw writes
instead. Three implementations of one idea was the symptom; the import graph
was the cause.

The alternatives were worse. A package under `services/bdd-cli/pkg/` is
importable repo-wide, but points `scripts/` — the tooling that drives the
repository — at the product it drives. One copy per root is three copies again,
which is the thing being removed.

## What the linter holds, and the one choice that made it cheap

`forbidigo` matches identifiers, never call expressions: it can rule on which
door was used and never on what went through it. The `os.Stdout`, `os.Stderr`
and `os.Stdin` identifiers are therefore what is banned, rather than
`fmt.Fprint*`. That catches every terminal write at its binding site —
`cmd.Stdout = os.Stdout` and `slog.NewTextHandler(os.Stdout, …)` included —
while leaving the 48 `fmt.Fprintf(&buf, …)` sites that build strings entirely
alone. Banning `fmt.Fprint*` instead would have rewritten those 48 into
`WriteString(fmt.Sprintf(…))`, which is what staticcheck's `QF1012` tells you
not to write.

`analyze-types: true` is what defeats an aliased `import osx "os"`; verified by
running the rule against one.

`forbidigo` cannot express the second rule, because it is about a package, not
an identifier: `depguard` denies `pkg/console` to `scripts/**` outright. The
split it enforces is ANSWER versus NARRATION — a caller reads an answer
verbatim in a shape it fixed, and `scripts/` almost never produces one. Five
files are exempt, each holding a descriptor rather than a print: the lint
hook's stdin and its JSON verdict (Claude Code parses that stdout, and the
protocol is not ours to change), and three that wire a child process's
streams. Everything else there is `log/slog`, appending to one shared
`docs/history/tools.log.json` with a `tool` attribute naming its writer.

Two consumers were rewritten rather than exempted, which is what made the ban
possible: `/task-loop` reads the bound Ticket from `docs/history/state.jsonl`
with `jq` — state belongs in a fold-last-value store, not in a log — and
`scripts/merge`'s triage table is written to `tmp/merge/triage.md`, because
per-line log framing destroys a markdown table.

The same rule set already carried a dead pattern — `^fmt\.Errorf\([^,)]*\)$`,
which could never fire, because forbidigo never sees a paren. It is deleted;
`err113` is what forbids a `%w`-less `fmt.Errorf`, and always was.

## Why the lock is on the parent directory

`pkg/disk` takes a short advisory `flock` across each open-do-close, and
whole-file writes commit through a same-directory temp and a rename.

The lock cannot live on the target. A rename replaces the target's inode, so a
lock held on the old one is invisible to the next writer, who opens the new
one — meaningless for exactly the shape that most needs it.

It cannot live in a sidecar beside the target either. BDD fixtures assert on
file trees: `tests/bdd-cli/fixtures/*/cassettes/golden.json` records every
created path with its sha256, from a before/after snapshot. A `.lock` next to
its target lands in that diff. The repository already works around this
elsewhere — `spawn_log.go` times its log directory to fall outside the snapshot
window, and `harness_record.go` writes `harness.json` only after the post-run
snapshot.

An external lock directory keyed by a hash of the absolute path was rejected
for two failure modes that are silent rather than loud. `TMPDIR` is per-user on
macOS and this repository deliberately rewrites child environments
(`env -u CLAUDECODE`, per-fixture tmpdirs), so a parent and a child can resolve
different lock namespaces, take two different locks and both proceed. And a
periodic cleanup that unlinks a held lock file leaves the holder's descriptor
pointing at an unlinked inode while the next opener creates a fresh one at the
same path — exclusion gone, no error on either side.

The parent directory has none of that. Its inode is stable, nothing here
renames a directory, and `tests/libraries/fstree` records regular files only
(it returns `nil` for `entry.IsDir()`), so a hold is structurally incapable of
reaching a judged diff.

Measured, not assumed: 24 concurrent read-modify-writes through `disk.Update`
keep all 24; the same loop on bare `os.ReadFile`/`os.WriteFile` kept 1 to 4.

## Costs accepted, measured

There is no `fsync`. The rename is what a reader needs; a flush only adds
durability across a power cut, and it measured 9.5ms against 0.18ms per write —
fifty times the cost of everything else here put together.

What remains is a committed write at about 0.18ms against `os.WriteFile`'s
0.05ms, and a read at 0.021ms against 0.012ms: roughly seven syscalls where
there were three. At the scale this repository works in that disappears —
`go test -tags bdd ./tests/bdd-cli/ -mode=replay` runs in 51s either way, the
same as before every access in the tree went through here.

(An earlier draft of this file claimed 78s. That measurement was taken while
two other agents were saturating the machine, and it was wrong; the number
above is three uncontended runs.)

Unrelated files in one directory serialise against each other. Every held
section is a single open-do-close with no AI turn, subprocess or network inside
it, and the busiest directory is `docs/history/`, where serialising several
hook processes is the intent rather than the cost.

A process killed between create and rename leaves one staging file inside the
tree. The name is deterministic rather than random, so residue is bounded to
one per target and the next write of that target clears it; the realistic kill
is a fixture's own timeout, where the run is already red.
