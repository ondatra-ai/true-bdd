# TrueBDD (bdd-cli)

An aspirational **Spec-as-Source** CLI: Gherkin-style behavioural specs
plus a system-architecture description are the source of truth, and code
is a regeneratable build artifact. Today the tool operates one level
down — **Spec-Anchored** — driving Claude-mediated checklists over user
stories.

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

The test that distinguishes them: *Can you delete all the code and
regenerate it identically from the spec?* For Spec-as-Source the answer
is **yes by design**.

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
  user-visible behaviour, *not* code structure. Each scenario maps
  deterministically to an executable test.
- **Architectural spec** — services, data models, transport protocols
  (REST / GraphQL / etc.), endpoints, and the persistent contract
  (what survives a rebuild). Docker Compose YAML is a natural fit for
  the service shape.
- **Regeneration loop** — the AI is allowed to invent absent endpoints
  and code paths to satisfy the spec; what it *cannot* invent are the
  persistent contracts (data models, exposed endpoints) declared by
  the architecture.
- **BDD tests as oracle** — derived from the behavioural spec, they
  decide whether a regenerated build is acceptable.

`bdd-cli` is the substrate this vision is being built on. The `us`
subcommand suite manages the spec lifecycle; the `build` subcommands
(currently stubs) are where regeneration will live.

## Status

| Subcommand | State |
|---|---|
| `us create <id>` | **Working** — extracts a story from its epic, validates against the `us-create` checklist, writes to `docs/stories/`. |
| `us refine <id>` | **Working** — iterates a story against the `us-refine` checklist; updates in place. |
| `us apply <id>` | **Working** — walks every AC in a refined story, validates against `us-apply`, and merges scenarios into a central `requirements.yaml` registry. |
| `build tests` | **Stub** — will generate executable tests from the Gherkin scenarios in the registry. |
| `build code` | **Stub** — will regenerate code from the registry plus the architectural spec. This is the Spec-as-Source step. |

All `us` commands accept `--fix` for an interactive loop in which
Claude proposes edits for each failed check and the user accepts,
refines, or exits.

## Install

Requires Go 1.25 and the `claude` CLI on `$PATH`.

```bash
go build -o ./bdd-cli ./src
```

## Usage

The tool spawns `claude` as a subprocess. If you invoke it from inside
a Claude Code session, unset `CLAUDECODE` first so the child has a
clean environment:

```bash
env -u CLAUDECODE ./bdd-cli us create 4.1
env -u CLAUDECODE ./bdd-cli us refine 4.1 --fix
env -u CLAUDECODE ./bdd-cli us apply  4.1 --fix
```

`us refine` issues many sequential Claude calls and typically takes
~5 minutes end-to-end. Don't abort early.

## Configuration

The host project supplies a `bdd-cli.yaml` that pins the engine type,
filesystem paths, prompt-template paths, and the documents the
checklists are allowed to cite (PRD, architecture, coding standards,
glossary). See the worked example at
[`tests/bdd/fixtures/us-create-happy-path/input/bdd-cli/bdd-cli.yaml`](tests/bdd/fixtures/us-create-happy-path/input/bdd-cli/bdd-cli.yaml).

Prompt templates live in [`templates/`](templates/) (Go `text/template`
with sprig).

## Testing

```bash
# unit tests
go test ./...

# end-to-end BDD fixtures — real Claude calls, ~3–5 min per fixture
go test -tags bdd ./tests/bdd/...
```

Fixtures under `tests/bdd/fixtures/<scenario>/` are folders containing
`cmd`, `input/`, optional `answers` (stdin for `--fix` runs), and
`expected/{exit_code,stdout.regex,judge.md}`. The runner builds the
CLI, copies `input/` into a tmpdir, executes `cmd`, and asks Claude
to score the resulting diff against the `judge.md` rubric. The whole
suite skips if `claude` is not on `$PATH`.

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
tractable again — without giving up determinism by relying on free-form
prose specs.

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
