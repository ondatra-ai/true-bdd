# Pin generated writer configuration at the Scenario boundary

## Gap

On 25 October 2026, generation 1 of the #181 cut A drill rebuilt an engine that passed 252/252 Scenarios. Generation 2's real codex-cli 0.162.1 refused its generated config.toml: `network = false` under `[permissions.writer]` was a boolean where codex expected a network table. Request replay never parsed that configuration. Its hash covers argv and stdin, while generated codex homes are excluded from snapshots before traversal to protect authentication. Claude's referenced settings content is outside that hash too. Live configuration probes under `services/` disappear in a cut.

## Decision

The architecture states every generated codex configuration key, permission-table entry order and shell environment value, and Claude settings keys, hook command and permission arrays. Two Registry Scenarios, `writer-config-001-codex` and `writer-config-002-claude`, each drive `build code --fix` through one controlled writer turn and inspect the engine-generated file with the existing scratch-file matching step. Anchored patterns cover the complete contents, normalizing host/toolchain paths through regex-escaped {{CWD}}, {{GOROOT}} and {{GOMODCACHE}} substitutions in that existing step, substituting {{ENGINE}} with the engine executable and eliding only the hook run basename. Codex's optional installed runtime read entries and explanatory comments remain portable. The restricted filesystem map, ordered writes and six denials, network table, environment, hook and Claude deny rules remain literal assertions.

These assertions run in record, live and replay modes; they inspect files the engine generated before the shim answers. They do not read credentials or make generated homes eligible for cassette snapshots. The fixtures are recorded individually through the declared record command with real codex and Claude. Existing recordings remain byte-identical.

## Alternatives and verification

Hashing referenced configuration in the shims would change existing request identities and require re-recording. Two new fixtures close the gap without that migration. Live sandbox probes remain necessary evidence that an installed CLI enforces the documented grants; the Scenarios ensure a rebuild preserves the form those probes calibrated.

The codex assertion is verified against the refused generation-1 file and by replay with a temporary engine mutation replacing the network table with `[permissions.writer]` and `network = false`. Replay still serves the writer's request, but the configuration assertion fails. Restoring the documented form passes. Claude's hook and deny assertions are similarly checked by mutation. No cassette is edited for these checks.

Decided by the Owner in #209, following #199's two-generation verification. No separate grilling entry decided this repair; the parent drill's replay and isolation decisions are recorded in `docs/qna/20261022-201524-rebuild-from-spec-drill.yaml`, Q5.
