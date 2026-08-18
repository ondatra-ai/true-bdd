# TrueBDD

An aspirational **Spec-as-Source** CLI: Gherkin-style behavioural specs
plus a system-architecture description are the source of truth, and code
is a regeneratable artifact whose *observable behaviour* — not its
byte-for-byte shape — survives a rebuild. Today the tool operates one
level down — **Spec-Anchored** — driving Claude-mediated checklists over
user stories.

## Table of contents

- [Background — the three levels of SDD](#background--the-three-levels-of-sdd)
- [Vision](#vision)
- [Status](#status)
- [Install](#install)
- [Usage](#usage)
- [Configuration](#configuration)
- [Testing](#testing)
- [How it compares](#how-it-compares)
- [References](#references)
- [Contributing](#contributing)
- [License](#license)

## Background — the three levels of SDD

The de-facto taxonomy in the SDD literature (arXiv 2602.00180,
Piskala 2026) splits spec-driven development into three patterns:

| Level | Source of truth | Code edits | Canonical tools |
|---|---|---|---|
| **Spec-First** | Code (after release). Spec only starts the process. | Hand-edited. | Cursor + rules, early Spec Kit. |
| **Spec-Anchored** | Code, but spec is a living contract. CI validates code against spec. | Hand-edited; spec updates via review. | GitHub Spec Kit, Kiro, BMAD, OpenSpec, current Tessl, LeanSpec, Augment Intent. |
| **Spec-as-Source** | Spec. Code is derived. | Forbidden — edit the spec, regenerate the code. | Tessl (historically, via `tessl build`), **TrueBDD** (aspirational). |

The test that distinguishes them: *Can you delete all the code,
regenerate from the spec, and have the new build pass every behavioural
assertion the spec carries?* For Spec-as-Source the answer is **yes by
design**. The regenerated code is free to differ in structure, naming,
even in which auxiliary endpoints exist — what survives a rebuild is
the **behaviour**, not the bytes.

## Vision

Pre-AI, code was inevitably the only authoritative artifact: developers
discover detail mid-implementation, work around third-party limits,
absorb UAT feedback, and the spec quietly diverges. The arrival of
coding agents doesn't fix this — agents drift too, and a stakeholder
can still ask for a checkbox where a dropdown was specified. Code
remains the only place the system's real behaviour is pinned.

Spec-as-Source flips that contract. To make it work, the spec system
has to carry enough information for an AI to reconstruct the code:

- **Behavioural spec** — Gherkin (or a Gherkin-shaped DSL) describing
  user-visible behaviour, *not* code structure. Every scenario becomes
  an executable test that any regenerated build must pass.
- **Architectural spec** — services, data models, transport protocols
  (REST / GraphQL / etc.), endpoints, and the persistent contract
  (what survives a rebuild). Docker Compose YAML is a natural fit for
  the service shape.
- **Regeneration loop** — the AI is allowed to invent absent endpoints
  and internal code paths to satisfy the spec, so two rebuilds from
  the same spec will not produce byte-identical code. What it *cannot*
  invent are the persistent contracts (data models, exposed endpoints)
  declared by the architecture — those are pinned across rebuilds.
- **BDD tests as oracle** — derived from the behavioural spec, they
  are the *definition* of a correct rebuild. If every scenario passes,
  the rebuild is acceptable, regardless of how the code differs from
  the previous build.

The contract Spec-as-Source promises, then, is **behaviour preservation
under regeneration** — not byte-identical regeneration. The behavioural
spec defines observable outputs; the architectural spec pins the
persistent contracts; the implementation in between is free to drift
between rebuilds, as long as both contracts are honoured.

`true-bdd` is the substrate this vision is being built on. The `us`
subcommand suite manages the spec lifecycle; the `build` subcommands
implement the regeneration loop — `build tests` derives executable
tests from the registry, `build code` regenerates production code
until those tests pass.

## Status

| Subcommand | State |
|---|---|
| `us create <id>` | **Working** — extracts a story from its epic, validates against the `us-create` checklist, writes to `docs/product/stories/`. |
| `us refine <id>` | **Working** — iterates a story against the `us-refine` checklist; updates in place. |
| `us apply <id>` | **Working** — walks every AC in a refined story, validates against `us-apply`, and merges scenarios into the central `docs/scenarios.yaml` registry. |
| `build tests` | **Working** — walks every scenario in the registry against the `build-tests` checklist and exits non-zero if any scenario is not executable: every one of its steps must bind to a step definition in the suite that owns it. With `--fix`, failed cells drive a Claude-mediated authoring loop that writes the missing step definition; the registry itself is never modified. |
| `build code` | **Working** — walks every suite declared under `testing.suites[]` in the architectural spec (`architecture.yaml`), discovers currently-failing tests via each framework's runner, and exits non-zero if any remain. With `--fix`, each failure drives a Claude-mediated turn that edits production source until the test passes; test files and the registry are never modified. This is the Spec-as-Source step. |

Every command accepts `--fix` for an interactive loop in which
Claude proposes edits for each failed check and the user applies,
refines, or exits. `build tests` also takes `--requirements <path>`
and `build code` takes `--architecture <path>` to override the
configured spec locations (`documents.scenarios_yaml` and
`documents.architecture_yaml` in `true-bdd.yaml`).

## Install

Requires Go 1.25 and the `claude` CLI on `$PATH`.

```bash
mkdir -p ./bin && go build -o ./bin/true-bdd ./services/bdd-cli
```

## Usage

The tool spawns `claude` as a subprocess. If you invoke it from inside
a Claude Code session, unset `CLAUDECODE` first so the child has a
clean environment:

```bash
env -u CLAUDECODE ./bin/true-bdd us create 4.1
env -u CLAUDECODE ./bin/true-bdd us refine 4.1 --fix
env -u CLAUDECODE ./bin/true-bdd us apply  4.1 --fix
env -u CLAUDECODE ./bin/true-bdd build tests --fix
env -u CLAUDECODE ./bin/true-bdd build code  --fix
```

`us refine` issues many sequential Claude calls and typically takes
~5 minutes end-to-end. Don't abort early.

## Configuration

The host project supplies a `true-bdd/` directory at its root:

- `true-bdd.yaml` — the model tiers (below), filesystem paths under
  `paths:` (`epics_dir`, `stories_dir`, `checklists_dir`, `tmp_dir`,
  `tmp_glob`, and the `test_write_globs` roots `build tests --fix` may
  author into), per-command prompt-template paths, and a `documents:`
  map naming the files a check may cite (`product`, `architecture_yaml`,
  `scenarios_yaml` — the scenario registry). Each check in a checklist
  lists the document keys it needs under `docs:`, and the engine points
  the prompt at those files.
- `checklists/` — one checklist per command, named by hyphenating the
  command path (`us create` → `us-create.yaml`, `build tests` →
  `build-tests.yaml`, …).
- `*-schema.yaml` — Yamale schemas pinning the shape of the spec
  artifacts (stories, epics, the scenario registry, checklists,
  `architecture.yaml`); the host project validates them in its lint
  step, outside the engine itself.

### Model tiers

Every checklist cell runs up to three AI turns with genuinely different
needs: **validating** a `Q:` wants strong reasoning, turning a failure
into a **fix prompt** wants a mid model, and **writing the fix** wants a
cheap high-context coder. The best model for each may live behind a
different CLI, so the engine names three tiers and binds each to a
`"<cli>:<model>"` pair — then gives each of the three roles its own
default, because one fallback tier cannot serve turns this different:

```yaml
engine:
  models:
    xhigh: "claude:claude-fable-5"
    high: "claude:claude-opus-4-8"
    coder: "crush:zhipu-coding/glm-5.2"
  default_prompt_model: high
  default_fix_model: high
  default_apply_model: coder
```

The value splits on the **first** colon, so hyphenated and
provider-qualified model ids survive intact. Supported CLIs are
`claude`, `crush`, and `codex`; each must be on `$PATH`.

A checklist picks a tier per role, and any single prompt overrides it:

```yaml
engine:
  prompt_model: xhigh   # the validation turn
  fix_model: high       # failure → fix prompt
  apply_model: coder    # writes the file

sections:
  - id: test-passes
    validation_prompts:
      - Q: "…"
        model: high         # overrides prompt_model for this cell
        fix_model: coder    # optional
        apply_model: coder  # optional
```

Resolution runs **prompt → checklist → the role's engine-level
default**, so a checklist that names nothing still gets `coder` for the
turn that writes files and `high` for the one that validates. A tier
name that is not configured, an unknown CLI, a missing role default, and
a default naming an unconfigured tier are all startup errors — the
engine never silently substitutes a different model than the checklist
asked for.

**Permissions across CLIs.** Tool permissions are declared once, as the
engine's execution mode, and projected onto each CLI:

| | `claude` | `crush` | `codex` |
|---|---|---|---|
| System prompt | native flag | prepended to the prompt | prepended to the prompt |
| Write gate | `--allowedTools` | generated `PreToolUse` hook | `-s read-only` / `workspace-write` |

`crush run` has no permission gate of its own, so the engine generates a
per-run config (pointed at via `CRUSH_GLOBAL_CONFIG`, leaving any host
`.crush.json` untouched) whose `PreToolUse` hook is this binary's hidden
`true-bdd crush-guard` subcommand — a host project needs no guard script
of its own. Hooks are additive, so a host config's own hooks run
alongside it and a denial from either blocks the tool.

Two crush behaviours the engine works around, both verified against the
live CLI rather than assumed:

- crush **silently ignores an unknown model pinned in config**, falling
  back to global state. The model is therefore always passed as `-m`.
- crush **fails open** when a hook cannot be executed. The engine
  probes the guard before every crush turn and refuses to run if it does
  not deny, so enforcement can never disappear quietly.

codex's sandbox is coarser than the other two: `workspace-write` grants
the whole working root rather than a specific glob. Roles that must not
write run `read-only`, and the engine recovers each turn's result from
the response text rather than from a file the model wrote.

The host project's documents live under `docs/`:

- `docs/architecture/architecture.yaml` — the architectural spec that
  scopes `build code` (which test suites get walked, and how each one
  runs — see below), tells `build tests` which suite owns a scenario,
  and carries the BDD `vocabulary:`
  (allowed action verbs, forbidden action verbs, forbidden qualifiers)
  cited by the `us refine` checks.
- `docs/product/product.yaml` — the product document, including the `roles:` that
  `us create` validates a story's `as_a:` clause against. These are
  roles (responsibilities), not personas (invented individuals) — one
  person commonly holds two of them.
- `docs/product/epics/*.yaml` — epic files that `us create` extracts
  stories from.
- `docs/product/stories/*.yaml` — the stories `us create` writes and
  `us refine` / `us apply` read.
- `docs/scenarios.yaml` — the scenario registry `us apply` merges into,
  `build tests` walks, and the test suites RUN: each scenario is a test,
  its steps bound to code by the suite that owns it.

Prompt templates live in [`templates/`](templates/) (Go `text/template`
with sprig).

### How a test suite runs

The architectural spec has one `testing:` section for the whole project,
beside the services it exercises — how a project runs its tests is a
property of the project, not of any one service. `build code` does not
know how to run your tests; the spec says. Each declared suite carries a
`commands:` block with one complete command line per AI-dependency mode:

```yaml
architecture:
  testing:
    suites:
      - name: bdd-cli
        service: bdd-cli          # must name an entry in services[]
        path: tests/bdd-cli
        framework: go-test        # go-test | jest | playwright
        commands:
          record: "go test -tags bdd -json -count=1 ./tests/bdd-cli/ -mode=record"
          replay: "go test -tags bdd -json -count=1 ./tests/bdd-cli/ -mode=replay"
          live: "go test -tags bdd -json -count=1 ./tests/bdd-cli/ -mode=live"
  services:
    - name: bdd-cli
      path: services/bdd-cli
      language: go
```

The rules, all enforced at startup rather than discovered mid-run:

- **All three modes are mandatory.** The engine carries no built-in
  invocation to fall back on: a command the spec does not state is one
  no reader can audit. `build code` runs `replay` — no model, no cost;
  `record` and `live` are reserved for commands that do not exist yet.
- **Each command is complete and framework-native**, including the flag
  that makes its output machine-readable (`-json` for go-test, `--json`
  for jest, `--reporter=json` for playwright). Omit it and the runner
  parses prose, finds no failures, and the walk goes falsely green —
  so a command without it is refused.
- **Quoting is honoured, nothing else.** `-run '^TestGreen$'` survives
  as one argument; there is no expansion, substitution or piping. A
  command needing a shell belongs in a script the spec then names.
- **The working directory** is the one holding the suite's `config:`
  file — which is what lets `npx` resolve a suite's own local install.
  A suite that declares no `config:` inherits the directory `true-bdd`
  itself was run from, so its command should be written relative to the
  repo root and the CLI run from there.
- **`service:` names one entry in `services[]`**, and that service's
  `path:` is the only root `build code --fix` may write. A name that
  resolves to nothing grants nothing, so it is refused at startup rather
  than discovered as a fix that never lands. A suite covering two
  services is two suites.
- **Re-running one test** appends only a name filter (`-run`,
  `--testNamePattern`, `--grep`) to that same command, so a rerun keeps
  the build tags and flags that made the test discoverable.

## Testing

```bash
# unit tests
go test ./...

# end-to-end BDD scenarios — real Claude calls, ~3–5 min per scenario
go test -tags bdd -timeout=180m ./tests/bdd-cli/...
```

The suite's contents come from `docs/scenarios.yaml`: every scenario the
architectural spec assigns to it gets one top-level Go test named after
its id (`TestE2E016`), written into the file the scenario's `path:` names
by `true-bdd build tests --fix` and by nothing else. Its steps are bound
by regexp to the definitions in `tests/bdd-cli/steps/`. A scenario's
Given step names the fixture tree it drives.

**A fixture is a directory, not a document — there is no manifest.**
Trees under `tests/bdd-cli/fixtures/<name>/` hold the designed
host-project content under `input/` (`docs/` at minimum, plus project
sources, a per-fixture `CLAUDE.md`, or engine-config overrides under
`true-bdd/` when the scenario needs them) and the recording under
`cassettes/`. What the run DOES — the invocation, the exit code, the
stdout patterns, the interactive answers, the clauses it is judged
against — all comes from the scenario. Three optional files sit at the
fixture root beside `input/`: `prep.sh` (run in the tmpdir before the
pre-run snapshot, e.g. `npm install`), `teardown.sh` (best-effort
cleanup after the post-run snapshot, e.g. stopping a compose stack), and
`checklist-prompts.yaml` (which prompts of a shipped checklist the walk
evaluates). The runner builds the CLI, pre-populates a tmpdir with the
live engine ingredients (checklists and prompt templates), overlays the
fixture's input tree, snapshots, runs what the scenario's When step
names, and — in live and record only — asks Claude to score the
resulting diff against the scenario's `judge:` clauses. In live and
record a scenario skips when `claude`, or any CLI a model tier names, is
not on `$PATH`; under `-mode=replay` nothing skips, because no model is
spawned at all.

## How it compares

Within the Spec-Anchored tier, comparable projects: **Spec Kit**
(GitHub) leans on a `constitution.md` and a four-phase workflow;
**Kiro** (AWS) bundles specs with steering files inside an agentic
IDE; **BMAD-METHOD** is a 12-role multi-agent framework; **OpenSpec**
treats every change as a spec proposal needing approval; **Tessl**
operates a spec registry over MCP; **LeanSpec** keeps living docs
under 2K tokens with a `validate` command. Of these, only Tessl ever
shipped a true Spec-as-Source mode (`tessl build`, retired Jan 2026).

TrueBDD's bet is that **Gherkin-grade behavioural specs + an
explicit architectural contract** are enough to make Spec-as-Source
tractable again — pinning observable behaviour and persistent
contracts tight enough that the regenerated code's shape can vary
freely between rebuilds without the system's observable behaviour
drifting.

## References

- Piskala, *Spec-Driven Development: From Code to Contract in the Age
  of AI Coding Assistants*, arXiv 2602.00180 (Feb 2026).
- *Constitutional SDD*, arXiv (Feb 2026).
- Augment Code, *6 Best Spec-Driven Development Tools* (Mar 2026).
- ThoughtWorks Technology Radar Vol 33 (2025).
- GitHub Spec Kit — https://github.com/github/spec-kit
- Kiro — https://kiro.dev
- BMAD-METHOD — https://github.com/bmad-code-org/BMAD-METHOD
- OpenSpec — https://github.com/Fission-AI/OpenSpec
- Tessl — https://tessl.io
- LeanSpec — https://lean-spec.dev

## Contributing

Pre-1.0 and direction-finding. Open an issue before non-trivial work.
Lint with `golangci-lint run` against `.golangci.yaml`.

## License

TBD. Not yet released for external use.
