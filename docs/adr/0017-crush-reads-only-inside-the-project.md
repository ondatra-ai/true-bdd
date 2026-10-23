# A crush turn reads only inside the project

The write guard that every crush tool call passes through also confines crush's local reads (`view`, `ls`, `grep`, `glob`, `diagnostics`) to the project root, the directory `true-bdd` was started in, as Claude Code already confines claude turns. Until now a read was allowed whatever it read. In #181's iteration 1 a crush apply turn used that twice to copy the deleted `tests/bdd-cli/steps/` into the project: once from the checkout above its worktree, once by absolute path from a scratch directory. A host's build must not depend on files no Specification document describes, and no turn may carry a neighbouring project's files into a host. So this is a product rule, not only the drill's.

## Considered Options

- Also allow toolchain roots (the Go module cache, `GOROOT`) or extra roots configured in `true-bdd.yaml`: not now. A host that needs one asks for it in its own ticket.
- Enforce the boundary only in the drill, from outside the engine: the leak affects every host, not only a rebuild.

## Consequences

- The network tools (`fetch`, `sourcegraph`, the Context7 tools) keep today's behaviour and remain a channel to outside sources. The drill audits them, and there a fetch of TrueBDD's own code counts as contamination.
- `TRUE_BDD_CRUSH_POLICY` carries a `read_root`, and a policy without one is refused. Symlinks resolve on both sides, a relative read path resolves against the read root, and a read naming no path is allowed.
- Codex turns (their sandbox) and claude turns (Claude Code) are unchanged.

Decided by the Owner in the triage of #187 (<https://github.com/ondatra-ai/true-bdd/issues/187#issuecomment-6367974669>). No grilling entry records it.
