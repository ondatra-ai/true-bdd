# Remove crush from the product and use codex for the coder tier

The Owner decided in #203, following #181's 24–25 October writer trials, to remove crush from the product and bind `engine.models.coder` to `codex:gpt-6.1-sol`. `default_apply_model` remains `coder`. Codex rebuilt cut A in 47 minutes and passed 253/253 Scenarios; Sonnet 5.5 took 61 minutes and passed 256/256. The crush GLM writer hit its five-hour usage cap after 97 minutes and passed 140/245. The Owner will have no GLM subscription.

## Decision

The product has only claude and codex providers. A crush model reference is refused at startup by the same unknown-CLI validation as any other unsupported name. The provider, shim, hook, configuration, watch and provider-specific Scenarios are removed. Shared canonical replay matching, write-root and symbolic-link algorithms remain under provider-neutral names for the Claude guard. Started codex and Claude writer failures retain edits and follow the same counted rerun; the crush log response watch is retired.

ADR-0017, ADR-0018 and ADR-0020 are superseded by this ADR. They remain as historical decisions. ADR-0019's failing-set lifecycle, ADR-0021's codex writer and ADR-0022's Claude writer remain in effect as amended here. The coder tier changes every default apply turn; plan and validation turns retain `read-only`. Only build code's writer receives canonical test-command forms.

## Confinement and deferred guarantee

**Resolved by [ADR-0024](0024-confine-every-codex-apply.md).** The migration limitation and evidence below describe this decision's original state.

Removing the crush guard loses ADR-0017's guarantee that every turn of every command reads only inside the project. Build code's codex writer keeps ADR-0021's generated home, permission profile, credentials cleanup, offline toolchain, read boundary and service/scratch writes. The Claude writer keeps ADR-0022's guard. Other codex apply turns keep their current sandbox until #205 supports their document files and configured test globs. No broader confinement is claimed here. Cuts B to D of #181 wait for #205; cut A uses build code and can run.

## Recording blocker found during migration

The ticket described the other codex apply sandboxes as workspace-write. The source and live recording show a narrower distinction: source-edit turns such as build tests use workspace-write, but scratch-only EditMode uses read-only. At `us-apply-002-duplicates-collapse`, codex-turn004 logged `sandbox=read-only` and could not edit the scratch Registry, returning `applied: false`; the recording was rejected. The default coder therefore exposes an existing unsupported document-edit combination. Scenario and term-document fixes have the same scratch-only mode; Story replies that return their content need no tool write.

The Owner explicitly required retaining today's sandboxes until #205. This change does not widen them, add an automatic fallback, falsify a confirmation, or weaken the Scenario. Ordered recording stops at this fixture. #205 must provide writable confined scratch before this part of the migration can finish. A person can explicitly select an existing Claude apply tier in the meantime. The recording manifest and continuation report retain the failure evidence.

## Evidence and alternatives

The writer trials and the plan-reading findings are recorded in #181, #199 and #203. Keeping crush would retain a provider the Owner cannot use. Applying the build-code directory profile to all commands would misinterpret document files and globs, so #205 owns that design. #204 independently removes the repository tooling fallback and audit; this product decision does not modify that tooling.

No new grilling decided the removal: the Owner decided it in #203. The parent drill's accepted replay and isolation limits are recorded in `docs/qna/20261022-201524-rebuild-from-spec-drill.yaml`, Q5. Historical wrapper/channel placement is recorded in `docs/qna/20261021-112516-self-spec-and-doc-validation-chain.yaml`, Q19 and Q33.
