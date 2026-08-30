# alint as the lint dispatcher

**Status:** researched and designed 2026-08-29, nothing started. Supersedes nothing;
`scripts/lint` works today. Verified against alint 0.15.2 (the version CI pins) on
this tree — every claim below carries the experiment that produced it.

## Verdict

Adopt alint as the dispatcher, not as the linter. `.alint.yml` becomes the one table
that says *which gate runs on which path*; each external linter enters through a
`command:` rule whose argv is a thin Go leaf (`go run ./scripts/cmd/lint <gate>`), and
that leaf keeps owning the tool's argv through `pkg/cli/<tool>`.

That deletes `scripts/lint/dispatch.go` — `selectGates`, the extension switch, the
`go.mod` sentinel walk — and slims `hook.go` to payload-in, verdict-out. It does not
delete the gates themselves, and it should not: two of them hold knowledge YAML cannot
express, and one of those is the reason the hook is readable at all.

**The in-process `pkg/alint` the task sketched cannot exist.** alint is a Rust binary
(`Mach-O 64-bit arm64`); there is no Go API to link against. The signature that sketch wanted — `Check(ctx, scope) error` — is
still the right seam, just implemented as spawn-and-decode in `pkg/cli/alint`.

## What alint can and cannot do

Every line here was run, not read.

| Capability | Finding |
| --- | --- |
| Scope to one file | **No.** `alint check <file>` refuses: "is a file, but `check`/`fix`/`baseline` take a repository root (a directory), not a single file". |
| `--changed` | `git ls-files --modified --others --exclude-standard`. A **staged-only** file is invisible to it — verified by staging an edit and watching the rule spawn nothing. `--base <ref>` uses the three-dot diff instead. |
| `{{env.X}}` in config | Works in value fields, including `paths:`. Resolved at **config-load** time, so an unset var without `\| default('…')` is a hard error even when the rule's `when:` is false. |
| `when:` expressions | Resolved at evaluation; unset env is `null`/falsy, so bare `when: "env.SCOPE"`, `when: "not env.SCOPE"` and the compound `not A and not B` all work. Operators are comparison plus `and`/`or`/`not` only — `endswith` is a parse error, so extension routing cannot live here. |
| Scope a glob rule to named files | **Yes**, `scope_filter.include_manifest_paths` — `paths:` supplies the extension, the manifest supplies the files, and the rule fires on the intersection. Fields: `source`, `extract`, `derive_target`, `expect_nonempty`. |
| Manifest path resolution | Entries resolve **relative to the manifest's own directory**, not the repo root. A manifest at `tmp/` must therefore list `../scripts/lint/dispatch.go`. Undocumented; found by watching a subdirectory manifest match nothing. |
| Missing or empty manifest | Warns, matches nothing, exits 0. Never a hard failure. |
| `command` kind | One process per matched file, cwd = repo root. Substitutes `{path} {dir} {stem} {ext} {basename} {parent_name}`; exports `ALINT_PATH`, `ALINT_ROOT`, `ALINT_RULE_ID`, `ALINT_LEVEL`. Exit 0 passes; non-zero is one violation whose message is combined stdout+stderr, truncated at ~16 KB (measured: 16405 characters, 276 lines). Default timeout 30 s. |
| `message:` on a `command` rule | **Replaces the tool's output entirely.** Omit it, or the finding never reaches the reader. |
| `command_idempotent` kind | Runs **once**, whole tree, takes no `paths:`. `files_from: stdout\|stderr` plus `files_pattern` (capture group 1 = path) turns output into per-file violations. Default timeout 120 s. |
| Trust gate | Both kinds load only from the top-level `.alint.yml`; a ruleset pulled in by `extends:` cannot spawn. |
| `alint fix` | Runs `command` rules too — reports them "no fixer", still exits 1. Exit codes are 0 pass / 1 violations. |
| Machine formats | `check` accepts `agent`; **`fix` rejects it** — human, json and markdown only. `--format json` is therefore what both are read as, and it is the richer shape anyway: `check` gives `results[].violations[].{path,message,line,column}`, `fix` the same under `items[]` plus a `status` of applied/skipped/unfixable/failed. `agent`'s `agent_instruction` carries no line or column. |
| Stream separation | stdout is pure JSON; the walker's warnings (a missing manifest, for one) go to stderr. Capture them apart — a combined sink corrupts the decode the first time alint warns. |
| `alint lsp` | Publishes diagnostics on open/save. Rejected: a long-lived server behind a one-shot hook buys nothing the CLI does not already give in 47 ms. |

Timings on this tree, warm: alint's own structural rules over the whole repo **0.49 s**;
alint's own overhead on a one-file scope, trivial command, **0.047 s**; golangci-lint one
package **0.67–1.15 s**, whole module **5.3–6.9 s**; markdownlint-cli2 one file
**0.16 s**; a `go run` leaf **~0.2 s**. Today's hook on one `.go` file is **1.35 s**;
the prototype composing alint → Go leaf → markdownlint on one `.md` file is **0.63 s**.
Latency is not a reason to choose either way.

## Why the linters still go through Go

The obvious shape — `command: ["golangci-lint", "run", "--fix", "{dir}"]` — was built and
run against a planted `declared and not used: x`. It works. What it costs is measurable,
and smaller than a first reading suggests: golangci writes its exclusion-rule bookkeeping
to stderr, alint concatenates stdout and stderr, and the violation message comes back
**2559 characters, of which 2290 (89%) are chatter** — about nine times the finding.

The finding is *not* buried, and an earlier draft of this document was wrong to say so.
golangci prints findings first and its bookkeeping last, so the real line lands third:

```text
`golangci-lint` failed (exit 1):
scripts/lint/claude_md.go:1: : # github.com/ondatra-ai/true-bdd/scripts/lint …
scripts/lint/zz_proto_tmp.go:4:2: declared and not used: x (typecheck)     ← the finding
package lint
1 issues:
* typecheck: 1
level=warning msg="The linter 'gomodguard' is deprecated …                 ← 14 lines,
level=warning msg="[runner/exclusion_rules] Skipped 0 issues by rules: …      2290 chars
```

So this is a token-cost and 16 KB-budget argument, not a legibility one: ~600 tokens of
tail noise in the block reason of every hook run that fails, multiplied by the number of
violating files. Spawning golangci straight from YAML is a viable design if that cost is
acceptable — it is not a disqualifier.

What decides it is that the cost is already paid for. `pkg/cli/golint` is twenty lines
that drop every `level=warning`, written for exactly this reason ("golangci prints nine
lines of exclusion-rule bookkeeping per run, which buries the finding in what the edit
hook hands back"). Discarding a filter that exists, to move argv out of the package ADR
0005 says owns it, buys nothing. And two of the remaining gates need a Go leaf whatever
golangci does — `markdownExclusions` reads the vendored skill names out of
`VENDORED-mattpocock.md` at runtime, `Schemas` reads the `documents:` map out of
`true-bdd.yaml` — so the leaf pattern is not a layer invented for golangci's benefit.
`.alint.yml` already uses it for `claude-md-width-and-mirror`.

The division that falls out is clean:

- **alint decides *which* files a gate sees** — extension globs, tree exclusions, the scope manifest.
- **the Go leaf decides *how* the tool is run** — argv, chatter filtering, and the dynamic knowledge YAML has no way to hold.

## Why an env var at all — and what it should be called

Two things were checked before accepting one, because a config-native answer would be
better than an env var:

- **alint has no command-shaped fix operation.** The twelve ops are file-content edits — `file_create`, `file_prepend`, `file_trim_trailing_whitespace`, `file_strip_bom` and so on. None runs a CLI, so "let the rule declare `golangci-lint --fix` as its fixer" is not available; a `command` rule under `alint fix` reports "no fixer" and exits 1.
- **The child cannot tell which subcommand invoked it.** Dumping the environment from a `command` rule under `alint check` and under `alint fix` produced byte-identical output. All alint exports is `ALINT_PATH`, `ALINT_ROOT`, `ALINT_RULE_ID`, `ALINT_LEVEL`, plus `ALINT_VAR_*` / `ALINT_FACT_*`. There is no mode variable to read.

So the scope has to arrive from outside, and whether to fix has to be decided by whoever
sets it. An earlier draft encoded that in the variable's name (`TRUEBDD_FIX_SCOPE`),
which was the wrong place: **`--fix` is already visible in the rule's argv**, and the
invariant "only a fixing run gets a scope" belongs in the single caller that can enforce
it. `pkg/cli/alint` exposes `Check(ctx)` and `Fix(ctx, scope)`; only the second sets the
variable, so a check with a scope is unrepresentable rather than merely discouraged. The
variable is then just what it is — `TRUEBDD_SCOPE`, a path to a manifest.

## The config shape

Each linter becomes a pair, selected by that one variable.

```yaml
  # Scoped: fires only when the shim names a manifest. Fixes as it goes.
  - id: go-lint-scoped
    kind: command
    level: error
    when: "env.TRUEBDD_SCOPE"
    timeout: 300
    paths:
      include: ["**/*.go"]
      exclude: ["tests/legacy/**", "tests/bdd-cli/fixtures/**", "services/bdd-web/**"]
    scope_filter:
      include_manifest_paths:
        source: "{{env.TRUEBDD_SCOPE | default('tmp/alint-scope.txt')}}"
        extract: { lines: {} }
    command: ["go", "run", "./scripts/cmd/lint", "go-package", "--fix", "{dir}"]

  # Full: one invocation, check-only, never on CI (the action has the cache).
  - id: go-lint-all
    kind: command_idempotent
    level: error
    when: "not env.TRUEBDD_SCOPE and not env.CI"
    timeout: 600
    command: ["go", "run", "./scripts/cmd/lint", "go-package"]
    files_from: stdout
    files_pattern: '^([^:\s]+\.go):[0-9]+'
```

The fix/check duality `scripts/lint/doc.go` calls load-bearing survives structurally
rather than by convention: the fixing rule cannot fire without a scope, and the
scope-less rule has no fixer to fire.

These rules may live in `.alint.d/50-linters.yml` rather than in `.alint.yml` itself —
drop-ins are trust-equivalent to the main config, so they may declare `command` rules
(a ruleset pulled in by `extends:` may not). That keeps `.alint.yml` about repository
shape and the linter wiring beside it.

## The hook shim

`scripts/lint/hook.go` is gone; `scripts/cmd/alint_hook` is what replaced it, and it is a
closure plus a chdir:

1. `hooks.PostToolUse(judge)` — `pkg/claude/hooks` reads the payload off its own stdin, takes `tool_input.file_path`, and writes the verdict on its own stdout. The command names neither descriptor.
2. `judge` makes the path repo-relative and drops anything ignored or outside the tree — the only part that came across from the old gate.
3. `alint.Fix(ctx, paths)` — `pkg/cli/alint` writes the manifest, spawns `alint fix --format json` with `TRUEBDD_SCOPE` set, and decodes stdout.
4. `Report.Outstanding()` empty → return nil, and nothing is written at all. Otherwise those findings are the returned error, whose message becomes the `reason`.

The manifest name carries the pid — `tmp/alint-scope-<pid>.txt` — because the env var
*is* the path, and two concurrent sessions (or a subagent sharing the worktree) would
otherwise race on one file. Today's hook passes paths as argv and has no such state.

`pkg/cli/alint` is the new wrapper — `pkg/cli/linters` already names the binary
(`linters.Alint`), so this is splitting that constant into its own package for the same
reason `golint` split out: the agent-JSON shape is knowledge about alint.

## Gate-by-gate disposition

| Gate | Today | After |
| --- | --- | --- |
| repository shape | `.alint.yml`, run by `Dispatch` | unchanged; now the entry point rather than one step |
| CLAUDE.md width + mirror | `command` rule → `lint claude-md` | unchanged — this is the pattern the rest adopts |
| golangci-lint | `golangci()` picks packages via `addGoPackage` | rule pair → `lint go-package` (new leaf over `pkg/cli/golint`) |
| markdownlint | `Markdown()` globs and excludes | rule pair → `lint markdown` keeps the vendored-skill exclusions |
| comments | `Comments()` walks `git ls-files` | rule pair → `lint comments`; alint supplies the paths, the leaf keeps the budget scanner |
| yamale schemas | `Schemas()` maps documents to schemas | `command_idempotent` → `lint schemas`; the `documents:` mapping stays in Go |
| dispatch + gate selection | `dispatch.go` (174 lines) | **deleted** — `paths:` globs replace it |

## What gets worse

1. ~~**The `go.mod` sentinel walk becomes a static exclude list.**~~ Avoided in the build: the walk moved to `lint.GoPackage` as `fenced()`, so a sentinel subtree is still discovered at runtime and `.alint.yml` does not have to enumerate them.
2. **Per-file spawning does not dedupe by package.** Five edited files in one package means five golangci runs on that package. Irrelevant for the hook, which names exactly one file; the commit gate takes the whole-module path instead. Only a scoped multi-file run pays it.
3. **Two rules per linter.** Roughly twelve lines of YAML each, and the `when:` pair has to stay mutually exclusive by hand.
4. **One more required PATH tool in the critical path.** alint already gates CI, but after this it also gates every edit. Pin the version — CI pins `@asamarts/alint@0.15.2`; `scope_filter` needs ≥ 0.9.6. The pin is current, not behind: 0.15.2 is the latest release, and the docs' "89 rule kinds" is 78 canonical kinds plus 11 aliases.
5. **The 16 KB message cap.** With `pkg/cli/golint`'s filter the findings fit; without it, the prototype's chatter alone spent ~2 KB per violation.
6. **Two claims above are designed, not run.** The `files_pattern` extraction never fired — the full-run prototype passed with 0 issues, so nothing was there to parse — and the composed `go-lint-all` → Go leaf → filtered stdout → `files_from` chain was never exercised end to end. Both degrade to one whole-run violation instead of per-file ones, so neither blocks the design; verify them at implementation time.

## CI — and why the full run was left alone

The design above proposed collapsing CI's four lint steps into one `alint check`, with
`command_idempotent` rules for the full pass. **That was dropped when it was built, and
the reason is `scripts/gates`.** The gate table carries a per-gate glob list that
`task-handle` narrows on — 2 s for a documentation ticket instead of 140 s — and a
conformance test pins that list to CI in both directions. Collapsing four steps into one
deletes the narrowing and forces a CI restructuring nobody asked for.

Nothing was needed anyway. `pkg/alint` reads `ALINT_PATH` from the environment and
`--fix` from the argv, so a gate invoked with **neither** is the whole-repository check
the table already runs. `go run ./scripts/cmd/lint comments` from `scripts/gates/table.go`
and `lint comments` from a scoped alint rule reach the *same closure*. So: no CI edit, no
`not env.CI` guard, no double-run to design around — and the untested `files_pattern`
extraction stopped being load-bearing, because no `command_idempotent` rule was written.

## eslint

Nothing to wire. The only eslint config in the tree is `services/bdd-web/eslint.config.mjs`,
and that subtree is fenced out of the root module by a sentinel `go.mod` and excluded from
alint's rules; `eslint` is not on PATH here. The tracked JS that alint *does* see —
`tests/libraries/reportserver/web/app.js` — has no eslint config at all. The `command`
rule shape covers eslint whenever the repository decides to lint that file; the design
does not add a gate for a linter this repository does not run.

## Decision log

- **2026-08-30** — Two claims corrected under challenge. The chatter does not bury the finding (golangci prints findings first); the honest figure is 89% of a 2559-character payload as a tail, which makes it a cost argument, not a legibility one. And `TRUEBDD_FIX_SCOPE` became `TRUEBDD_SCOPE` once it was established that alint has no command-shaped fix op and exports no mode variable — the invariant moved into `pkg/cli/alint`'s two-method surface. Also corrected: 0.15.2 is current, and the docs' 89 kinds are 78 plus 11 aliases.

- **2026-08-30** — Callers wired, and the scoped path is live. `scripts/cmd/alint_hook` drives `alint.Fix` from the PostToolUse payload; `.claude/settings.json` points at it; four `when: env.TRUEBDD_SCOPE` rules in `.alint.yml` route a file to its gates; `scripts/cmd/lint` is the closure they call. `scripts/lint/dispatch.go` and `hook.go` are deleted. Proven end to end by a live hook firing on a planted violation: the auto-fix applied, the finding blocked, and no `level=warning` chatter survived the chain. Latency 1.35 s, unchanged. **Only the scoped path moved** — see the CI section for why the full run stayed on the gate table.

- **2026-08-30** — `pkg/` built: `pkg/cli/alint` (Check, Fix, Report), `pkg/alint` (AlintLint and its closure), `pkg/claude/hooks` (PostToolUse). Both harnesses encapsulate the writer — a closure narrates through its logger and returns a finding, never a writer it was handed. ADR 0006 records the root split. The callers (`scripts/cmd/alint_hook`, the `lint` main, the `.alint.yml` rule pairs) are still to come.

- **2026-08-29** — Researched and designed. Adopt alint as dispatcher, keep linter argv in Go leaves. Rejected: an in-process `pkg/alint` (alint is a Rust binary), alint as the hook command itself (it speaks neither the PostToolUse payload nor the block verdict), `alint lsp` (a server behind a one-shot hook), and spawning linters directly from YAML (routes around `pkg/cli/golint`'s chatter filter and ADR 0005). Not started.
