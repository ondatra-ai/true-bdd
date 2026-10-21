# TrueBDD

TrueBDD (binary `true-bdd`) is a Go CLI that drives Claude-mediated checklists over a host project's specification documents. This repository is the engine, not a host project: its own `true-bdd/` and `docs/` are the seed the BDD fixtures run against. `README.md` carries the vision and the configuration reference; `GLOSSARY.md` the domain words.

## Layout

Go lives in four roots, and `.alint.yml` (`go-known-roots`) refuses a fifth:

- `services/bdd-cli/`: the engine, the product code.
- `tests/bdd-cli/`: what exercises it: generated scenario tests, `steps/` definitions, and `fixtures/` with their recorded cassettes.
- `scripts/`: this repository's own tooling, run as `go run ./scripts/cmd/<name>`.
- `pkg/`: what the other three share, the IO channels and `pkg/testkit/` (the BDD harness). A new package under it needs an ADR and an entry in `.alint.yml` (`pkg-is-the-declared-channels`).

Beside them: `docs/` holds the specification documents (`docs/scenarios.yaml` is the Registry the BDD suite is generated from), `true-bdd/` the engine's config, checklists and schemas, `templates/` the prompt templates, and `tmp/` gitignored scratch, BDD run dirs among it. Session-temporary files go in `./tmp/`.

## Conventions

Before you create or edit a file, read every `.claude/rules/*.md` whose `paths:` front matter matches its path, and follow it: each is the contract its area's gates enforce.

In engine code, an AI turn that must happen gets everything it needs: multi-stage generation, critique passes, full embedded context, whole prompts. This governs the engine's runtime model spend, not how much code you write.

## Checks

Claude Code lints every file it writes through a hook; any other agent runs the same checks itself:

- After editing: `go run ./scripts/cmd/alint <path>...` over the files you changed. It applies every fix it has and reports what needs a real edit; a Go finding may sit in a sibling file of the same package. Fix them all.
- Before you finish: `go run ./scripts/cmd/gates run --changed main` runs the gates your diff needs. CI runs all of them, and the pre-commit hook runs `--staged`.
- When you touch `services/`, `pkg/testkit/` or `tests/`: `go test -tags bdd ./tests/bdd-cli/ -mode=services:replay,tests:replay` replays every scenario offline in well under a minute. No gate runs it.
- Build: `go build -o ./bin/true-bdd ./services/bdd-cli`. Give every single-command `go build` an `-o ./bin/<name>`, since without it Go writes the binary into the current directory; to only compile-check, run `go build ./...`, which writes nothing.

## Hard rules

- Meet a gate by changing the code. `.golangci.yaml` and `.alint.yml` change only with the user's permission.
- Add new commits: `git commit --amend` and force pushes are off limits. Merge a pull request only when the user asks for that merge.

## Agent skills

### Issue tracker

Issues live in GitHub Issues. Read `docs/agents/issue-tracker.md` before you create, read, label or close one.

### Triage labels

Each triage role's label is the role's own name. Read `docs/agents/triage-labels.md` before you apply one.

### Domain docs

Single-context: `GLOSSARY.md` at the repo root, ADRs in `docs/adr/`. Read `docs/agents/domain.md` before you explore the codebase.
