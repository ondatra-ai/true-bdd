# A confined Claude build-code writer with a guarded replay

**Library access amended by [ADR-0028](0028-architecture-declares-access.md):** generated profiles now grant only exact architecture-declared Go libraries, their downloaded build graph and version metadata. The engine downloads them before build-code writers; no rule singles out an organization or the engine's module. The broad module-cache grants and organization exclusions below describe the original trial, superseded by ADR-0028. Claude's file tools follow the declaration; its replay and executed code remain unconfined with network, a known limit retained for model trials (Q15).

Only `build code --fix`'s Claude applier requests a generated writer configuration. [ADR-0025](0025-unbounded-ai-turns.md) extends its unbounded, cancellable lifetime to every role and CLI. Claude Code 2.1.285's `--restricted` mode, explicit `--tools` and working roots enforce file confinement independently of prompting. A `PreToolUse` hook invokes this engine's `claude-guard --policy <file>` for every exposed tool. Its Bash decision reuses ADR-0018's exact canonical matcher, with payload `cwd` and `tool_input.command`; `run_in_background` is denied. From the host root or the suite directory it also accepts exactly `cd '<suite dir>' && <canonical form>` (with shell-quoted directory), so Claude's Bash can reach a nested suite without a working-directory input. The writer's command forms include this prefix; Claude retains the suite directory after a successful Bash `cd`. No Bash permission pattern grants execution. Both Bash timeouts are 600000 ms.

## Isolation

Safe mode would disable the hook, so this turn replaces it with empty setting sources, strict MCP, disabled slash commands, explicit tools and generated settings. Environment switches disable discovered CLAUDE.md, automatic memory, bundled skills, workflows, plugin forwarding, marketplace auto-installation and background tasks. User/project settings and installed plugins contribute nothing. Auth and built-in plugins remain as with safe mode; no MCP server is configured.

A fresh `CLAUDE_CONFIG_DIR` was preferred but `claude auth status` reported signed out in the live probe. The writer therefore retains the existing sign-in without copying credentials. A fresh session UUID grants only its exact `projects/<encoded-host>/<session>/tool-results` directory, never historical sessions or other project entries. Extra file read roots are the installed Go toolchain and module cache, from `go env GOROOT GOMODCACHE`. The hook excludes `github.com/ondatra-ai`, its download-cache tree and all bare repositories under `GOMODCACHE/cache/vcs/` outside the host. File-tool paths and Glob patterns expand `~` and `~/…` before root checks. Searches outside the host reject ancestors of excluded module copies; Glob also checks its fixed pattern root and denies parent traversal. The host always takes precedence, including a checkout under `~/work` or the module cache. Symlinks resolve before the path checks. Writes reach only the execution mode's scratch and service roots, excluding the generated configuration directory so the writer cannot replace its own settings or guard policy. MultiEdit is guarded if present; 2.1.285 does not expose it in its built-in inventory.

Restricted mode and the hook govern file tools and commands, not the writer's code executed by the granted replay. ADR-0018's machine-permissions limitation remains. No new network or subprocess sandbox is claimed.

## Lifetime and outcomes

The router adds no deadline to any turn. Every role and CLI uses the caller's context and a process group; cancellation stops the turn and its descendants and stays fatal. Other Claude turns keep their arguments. There is no Claude response watch, time, turn, token or cost budget in any request, prompt, checklist or template.

A started Claude writer's non-zero exit, error result (including assistant rate-limit records and result API errors), missing envelope or empty answer preserves its files. Available stream cost and usage are logged even on failure. Only build code's applier recovers the typed outcome, warning once and running `PostNotApplied`; the existing allowance and terminal rerun behavior remain. Start failures, engine refusals and cancellation stay fatal. Writer templates select foreground completion for Claude and native process polling for codex.

The verification writer is `claude:claude-sonnet-5-5`, whose native reported context is 1000000 tokens, without a suffix or beta header. This is model capacity, not a claim that the probes filled the window.

## Alternatives and evidence

Scoped Write/Glob/MultiEdit permission patterns do not reliably define file boundaries. Denying all of `~/work` also denies the host fixtures. Restricted mode supplies the default boundary; the engine hook narrows additional roots and write grants. Prompt-only restrictions and broad transcript grants were rejected.

The Owner decided this writer trial in #202; the live probes and checks are recorded in the code run's report. No separate grilling recorded this choice. The parent drill's accepted replay and isolation limits are in `docs/qna/20261022-201524-rebuild-from-spec-drill.yaml`, Q5.
