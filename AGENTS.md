# TrueBDD

TrueBDD (binary `true-bdd`) is a Go CLI that drives Claude-mediated checklists over a host project's specification documents. This repository is the engine, not a host project: its own `true-bdd/` and `docs/` are the seed the BDD fixtures run against. `README.md` carries the vision and the configuration reference; `GLOSSARY.md` the domain words.

## Layout

The repository has three Go modules: the root (tests and scripts), `pkg/` (shared libraries), and `services/true-bdd/` (the engine). The root replaces `github.com/ondatra-ai/true-bdd/pkg` with `./pkg` for development; the engine has no local replacements. Go lives in four roots, and `.alint.yml` (`go-known-roots`) refuses a fifth:

- `services/true-bdd/`: the engine, all product code including its own IO channels and typed CLI wrappers under `internal/infrastructure/`. It imports no repository code outside this folder.
- `tests/true-bdd/`: what exercises it: generated scenario tests, `steps/` definitions, and `fixtures/` with their recorded cassettes.
- `scripts/`: this repository's own tooling, run as `go run ./scripts/cmd/<name>`.
- `pkg/`: what the tests and tooling share, their IO channels and `pkg/testkit/` (the BDD harness). A new package under it needs an ADR and an entry in `.alint.yml` (`pkg-is-the-declared-channels`). A wrapper only the engine spawns lives in the engine, under `services/true-bdd/internal/infrastructure/cli/`, so a rebuild regenerates it.

Beside them: `docs/` holds the specification documents (`docs/scenarios.yaml` is the Registry the BDD suite is generated from), `true-bdd/` the engine's config, checklists and schemas, `templates/` the prompt templates, and `tmp/` gitignored scratch, BDD run dirs among it. Session-temporary files go in `./tmp/`.

## Conventions

Before you create or edit a file, read every `.claude/rules/*.md` whose `paths:` front matter matches its path, and follow it: each is the contract its area's gates enforce.

In engine code, an AI turn that must happen gets everything it needs: multi-stage generation, critique passes, full embedded context, whole prompts. This governs the engine's runtime model spend, not how much code you write.

## Checks

Claude Code lints every file it writes through a hook; any other agent runs the same checks itself:

- After editing: `go run ./scripts/cmd/alint <path>...` over the files you changed. It applies every fix it has and reports what needs a real edit; a Go finding may sit in a sibling file of the same package. Fix them all.
- Before you finish: `go run ./scripts/cmd/gates run --changed main` runs the gates your diff needs. CI runs all of them, and the pre-commit hook runs `--staged`.
- When you touch `services/`, `pkg/testkit/` or `tests/`: `GOWORK=off go test -tags bdd ./tests/true-bdd/ -mode=services:replay,tests:replay` replays every scenario offline in well under a minute. No gate runs it.
- Module checks: from each of `.`, `pkg/` and `services/true-bdd/`, run `GOWORK=off go build ./...` and `GOWORK=off go test ./...`; root `./...` excludes nested modules. `go run ./scripts/cmd/gates build` and `go run ./scripts/cmd/gates test` cover all three.
- Build: `GOWORK=off go -C services/true-bdd build -o ../../bin/true-bdd .`. Give every single-command `go build` an `-o ./bin/<name>`, since without it Go writes the binary into the current directory; to only compile-check, run `go build ./...`, which writes nothing.

## Hard rules

- Meet a gate by changing the code. `.golangci.yaml` and `.alint.yml` change only with the user's permission.
- Add new commits: `git commit --amend` and force pushes are off limits. Merge a pull request only when the user asks for that merge.

## Agent skills

### Issue tracker

Issues live in GitHub Issues. Read `docs/agents/issue-tracker.md` before you create, read, label or close one.

### Triage labels

Each triage role's label is the role's own name. Read `docs/agents/triage-labels.md` before you apply one.

### Domain docs

Single-context: `GLOSSARY.md` at the repo root, ADRs in `docs/adr/`. Read `docs/agents/domain.md` before you explore the codebase. An ADR ends by naming the grilling Q&A log entries that decided it: the `docs/qna/<file>.yaml` path and each `Q<n>`.

## Codex

Codex writes the code and reads Claude's drafts as a second model through [common-codex](.claude/skills/common-codex/SKILL.md). This overrides a skill's instruction to spawn a Claude implementer, reviewer or exploration sub-agent; leave vendored skills unchanged. The session itself writes the brief, starts `go run ./scripts/cmd/codex` as a background task, waits and reads the summary. No Claude sub-agent stands over a Codex run.

- **Codex writes the code.** Every ticket or described code change is a code run in a new worktree on its own branch. Accepted findings, failing checks and merge conflicts on that branch are fix runs in the same worktree. An edit smaller than a handover (two or three calls on files Claude already holds) stays with Claude. There is no complexity exception: Claude takes the code task itself when Codex is unavailable or after two failed attempts. A run makes plain local commits on its branch and never pushes.
- **`mpc_implement-spec`'s implementers are code runs.** Claude merges completed branches into the integration branch when authorized. On conflict, abort and start a fix run on the ticket branch to merge the integration branch in, resolve and re-check; then merge again.
- **Readings run read-only.** `mpc_code-review`'s two reviewers are standards and spec review runs; Claude aggregates. A spec draft gets a plan-read before publication, a diagnosis before its fix, and a delegated decision a second reading before Claude carries it out. Code or public-source research is a research run with a precise question and criterion of done.
- **Claude keeps** the conversation with the Owner, decisions, specs, tickets, briefs, reading reports and diffs, the Owner's texts, and research through the Owner's connectors, which Codex lacks. Findings are weighed as valid, invalid or needing a test, never obeyed as orders.
- **When Codex is out.** The command checks sign-in and reports weekly usage before each attempt, warning past 90% without stopping. Tell the Owner before a batch past 90%. Signed out, unavailable sign-in, expired credentials or a usage limit ends a writer or reader `unavailable`; Claude does that writing or reading itself and says so. An `unavailable` run is not resumable, including after partial edits. Other failures get one resume by the recorded session id, or a fresh session with the saved brief when none exists. After the second failure Claude takes the work, naming both logs.

Run artifacts stay under ignored `tmp/codex/<run>/`; code worktrees under ignored `tmp/codex-worktrees/<run>/`. The command uses typed `pkg/cli/` operations, `pkg/disk`, `pkg/logging` and the process-group watchdog in `pkg/shell`. See the skill for briefs, flags, retry rules, completion checks and how to read the report.
