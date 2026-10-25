# Pin generated writer configuration at the Scenario boundary

## Gap

On 25 October 2026, generation 1 of the #181 cut A drill rebuilt an engine that passed 252/252 Scenarios. Generation 2's real codex-cli 0.162.1 refused its generated config.toml: `network = false` under `[permissions.writer]` was a boolean where codex expected a network table. Request replay never parsed that configuration. Its hash covers argv and stdin, while generated codex homes are excluded from snapshots before traversal to protect authentication. Claude's referenced settings content is outside that hash too. Live configuration probes under `services/` disappear in a cut.

## Decision

The architecture states every generated codex configuration key, permission-table entry order and shell environment value, and Claude settings keys, hook command and permission arrays. Two Registry Scenarios, `writer-config-001-codex` and `writer-config-002-claude`, each drive `build code --fix` through one controlled writer turn and inspect the engine-generated file with the existing scratch-file matching step. Anchored patterns cover the complete contents, normalizing host/toolchain paths through regex-escaped {{CWD}}, {{GOROOT}} and {{GOMODCACHE}} substitutions in that existing step, substituting {{ENGINE}} with the engine executable and eliding only the hook run basename. Codex's optional installed runtime read entries and explanatory comments remain portable. The restricted filesystem map, ordered writes and six denials, network table, environment, hook and Claude deny rules remain literal assertions.

These assertions run in record, live and replay modes; they inspect files the engine generated before the shim answers. They do not read credentials or make generated homes eligible for cassette snapshots. The fixtures are recorded individually through the declared record command with real codex and Claude. Existing recordings remain byte-identical.

## Alternatives and verification

Hashing referenced configuration in the shims would change existing request identities and require re-recording. Two new fixtures close the gap without that migration. Live sandbox probes remain necessary evidence that an installed CLI enforces the documented grants; the Scenarios ensure a rebuild preserves the form those probes calibrated.

The codex assertion is verified against the refused generation-1 file and by replay with a temporary engine mutation replacing the network table with `[permissions.writer]` and `network = false`. Replay still serves the writer's request, but the configuration assertion fails. Restoring the documented form passes. Claude's hook and deny assertions are similarly checked by mutation. No cassette is edited for these checks.

## Second gap: credential provisioning (#210)

Generation 1 after #209 passed 254/254 Scenarios but omitted copying auth.json. Generation 2 selected the file credential store in its valid configuration and failed with HTTP 401 at its first codex writer turn. Configuration shape alone did not hold the runtime preparation. This is the second instance of a live-only requirement disappearing in a rebuild.

Two more Scenarios observe the external CLI spawn before cassette effects, using fixture files containing the literal dummy object `{"true_bdd_dummy_auth":"issue-210-no-credentials"}` followed by a newline. The source homes also contain harmless config, instruction, history, session, skill, MCP and model-cache markers; copying the whole source home therefore fails the generated-home census even if config.toml is overwritten. Both replace only the external AI CLIs with deterministic fixture executables; the engine still performs preparation, starts those processes, archives events, handles failures and removes authentication. Recordings are made through the declared record command, separately per fixture. Existing hashes and recordings do not change. The shim's receipt contains no paths or authentication bytes, is excluded from per-call effects, and is independently required after the turn. A log line or a replayed receipt is insufficient.

| Architecture statement | Pin |
| --- | --- |
| Copy exactly auth.json, with 0600 permissions; generated home 0700; inherit no other home files | `writer-lifecycle-001-success` and `writer-lifecycle-002-failure`: shim inspects actual files at spawn, compares dummy bytes, checks types and modes and the home census |
| Remove authentication on success and on failed turns | Same two Scenarios: Then requires one observed spawn and no remaining auth.json; failure explicitly requires exit-7 recovery |
| Create absent edit roots before spawning | Same two Scenarios start without services/calc; shim requires it empty before effects and the stand-in cannot create it; `build-code-codex-003-missing-root` also pins the creation log and resulting service |
| Archive --json stdout separately from stderr, retaining both in the transcript | Same two Scenarios assert the complete JSONL sentinel and the independent stderr sentinel; argv hash pins --json and -o |
| Shell environment inherits nothing, with all stated PATH/Go/npm/Docker/OpenSSL values | `writer-config-001-codex` compares the complete shell policy; both lifecycle Scenarios additionally require private shell HOME, TMPDIR and GOCACHE directories before spawn |
| Protected generated home under the shared denied subtree | Lifecycle shim checks the resolved location and shared deny entry; `writer-config-001-codex` pins the full ordered permission profile |
| Disabled configuration discovery, network policy, high reasoning, context window, history and ephemeral CLI flags | `writer-config-001-codex` pins configuration; lifecycle cassette hashes pin CLI arguments |
| Actual native sandbox enforcement, OS/runtime read exceptions and optional installed runtime execution | Replay cannot execute codex's native sandbox or establish machine-dependent Mach-O/runtime behaviour. ADR-0021/0024 live probes calibrate enforcement; the configuration Scenario pins the engine-emitted policy. Its portable optional-runtime wildcard deliberately does not claim to test every installed-runtime admission/refusal branch. |
| Preparation-failure cleanup, cleanup-error reporting, concurrent symlink replacement, cancellation/descendant termination and absence of a router deadline | These require fault injection or live process supervision beyond the two completed-turn provisioning seams here; no lifecycle Scenario claims these outcomes. Existing provider/process tests and ADR-0024/0025 retain them. They are explicit residual limits, not inferred from a green credential Scenario. |
| Configured-glob post-turn enforcement, planning/validation and other commands' apply isolation | Independent engine behaviours covered by the existing confinement Scenarios and provider tests; these are not live-only provisioning and are unchanged here. |

A no-copy engine mutation must fail before cassette effects; retaining authentication only on failure must fail the failed-turn Scenario. The observer is opt-in and never examines an ordinary turn's authentication. All snapshots continue skipping generated homes before traversal. The new catalogue phrases use only existing `project tree` and `AI turn` terms, whose word mapping describes fixture-controlled setup and observation. The Specification divergence review found no new product behaviour: these Scenarios preserve the existing writer contract and document the remaining observation limits.

Decided by the Owner in #209 and #210, following #199's two-generation verification. No separate grilling entry decided this repair; the parent drill's replay and isolation decisions are recorded in `docs/qna/20261022-201524-rebuild-from-spec-drill.yaml`, Q5.
