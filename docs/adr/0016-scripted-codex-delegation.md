# Scripted Codex delegation uses typed shared CLI wrappers

Repository tooling delegates code and readings through `scripts/cmd/codex` and `scripts/codexrun`. Its typed operations live in `pkg/cli/codex`, `pkg/cli/crush`, `pkg/cli/gh`, `pkg/cli/git` and `pkg/cli/ps`; only those wrappers reach `pkg/shell`. They fit the existing `pkg/cli/**` channel allowance. The engine's Codex and self wrappers remain inside the engine: scripts cannot import them, and rebuilding the engine must not regenerate the tooling that builds it.

## Considered Options

- Import the engine's wrappers: violates the tooling dependency boundary and couples delegation to regenerated product code.
- Spawn generic commands from scripts: loses typed operations and violates the shell channel boundary.
- Keep a JavaScript runner beside the skill: adds a runtime and escapes the Go gates.

## Consequences

- The command stores attempts under ignored `tmp/codex/`, creates code worktrees under ignored `tmp/codex-worktrees/`, and proves writer completion with Git.
- `pkg/shell` supervises process groups; `pkg/disk` appends exact stream bytes without injecting record separators. Scripts narrate through `pkg/logging`, including their single closing summary.
- Issue #177 marks each writer agent's environment with `TRUE_BDD_CODEX_RUN=<name>:<attempt>:<sha256-of-checkout-root>`, inherited by Codex, crush and their descendants, including detached children. Typed `pkg/cli/ps` operations discover exact environment entries with `ps eww`, subtracting the command line so an argument cannot claim ownership. Before reporting an end, only that attempt's marked processes receive SIGTERM, a grace period and SIGKILL. Resume and a new writer refuse marked holders from all recorded attempts on the same worktree before consuming an attempt, naming the commands, PIDs and markers. Discovery or cleanup failure prevents the closing summary. Readers use process-group supervision only; neither readers nor writers select processes by working directory. Unmarked shells, editors and unrelated runs are never cleanup targets.
- No new root package category or lint-config exemption is needed. npm/pnpm installation, Areas and test databases from the reference are outside this repository's port.

Delegation is required by issue #175. The wrapper placement and channel boundaries follow `docs/qna/20261021-112516-self-spec-and-doc-validation-chain.yaml`, Q19 and Q33; those entries decided the engine-only placement and the explicit shell boundary, not this later delegation feature.
