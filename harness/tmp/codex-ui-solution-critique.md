# Final implementation review

Four MUST-FIX defects remain.

## MUST-FIX

### 1. The inventory request-size guarantee and terminal floor are incorrect

The declared minimum floor is 200 bytes ([budget.go:21](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/inventory/budget.go:21)), but the smallest normal floor with all fixed document chips is already 418 serialized bytes. `floorSnapshot` retains `DocumentErrors`, which is not bounded, and returns without rechecking its size ([budget.go:161](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/inventory/budget.go:161)).

The remote also subtracts a guessed 1,024-byte envelope ([agent.go:34](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/remote/agent.go:34)) and measures only the snapshot ([agent.go:414](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/remote/agent.go:414)), although the capped body includes folder, session, epoch, and token ([wire.go:118](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/remote/wire.go:118)). At a sufficiently small server cap, even the `limit_too_small` request cannot be uploaded; the remote eventually returns locally without promoting that state ([agent.go:317](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/remote/agent.go:317)). The smaller budget is also not persisted for later refreshes.

Concrete fix:

- Fit and verify the serialized full `InventoryRequest` using its actual folder/session/token envelope.
- Make the floor genuinely bounded: omit or cap error/base strings, calculate its real maximum, and preserve/render the planned global/per-state counts.
- Persist a successfully reduced budget.
- Report below-minimum availability out-of-band during registration/polling, or reject/clamp an impossible server configuration.
- Add real-route tests proving the final body is `<= limit`, including long/escaped paths and a cap below the minimum.

### 2. Scanning remains aggregate-memory-unbounded

`ScanWithBudget` builds every enriched epic/story first and only calls `fitSnapshot` after the entire folder is retained ([scanner.go:26](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/inventory/scanner.go:26)). Each story retains up to 256 KiB of raw plus its complete parsed content ([story_scan.go:134](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/inventory/story_scan.go:134)); every epic declaration also retains normalized declared content ([story_scan.go:79](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/inventory/story_scan.go:79)).

Thus many stories—or large parsed descriptions—can exhaust memory before the budget ladder runs. Repeated whole-snapshot marshaling during degradation further makes the path quadratic ([budget.go:91](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/inventory/budget.go:91)).

Concrete fix: maintain totals separately and fit incrementally as each file/row is processed. Once the floor is reached, continue scanning only to update bounded counters. Add an invariant or allocation test showing retained scanner state is bounded by the negotiated budget plus one currently decoded file.

### 3. A newer scan epoch can be silently lost behind a pending retry

A poll can advance `scanEpoch` and enqueue a refresh ([agent.go:225](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/remote/agent.go:225)). However, `buildOrReuseInventory` unconditionally returns an older pending request ([agent.go:344](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/remote/agent.go:344)). If that old retry then succeeds, the agent clears it and returns without checking whether a newer epoch arrived ([agent.go:302](/Users/peterovchinnikov/work/awesome/true-bdd/src/internal/app/remote/agent.go:302)).

Result: a manual refresh can be consumed server-side without ever scanning its newer ticket.

Concrete fix: after acknowledging a pending request, compare its epoch with the current `a.scanEpoch`. If a newer epoch exists, clear the old request and immediately perform a fresh scan for the latest epoch. Add a regression covering transient failure → newer refresh ticket → old retry succeeds → newer snapshot still uploads.

### 4. An open modal disappears instead of reporting a removed story

The page correctly re-derives the selected story from the newest snapshot ([page.tsx:178](/Users/peterovchinnikov/work/awesome/true-bdd/harness/app/sessions/[id]/page.tsx:178)), but it renders the modal only while both objects still exist ([page.tsx:268](/Users/peterovchinnikov/work/awesome/true-bdd/harness/app/sessions/[id]/page.tsx:268)). When the composite identity vanishes, the dialog is silently unmounted. The view-model’s `changedOnDisk` support exists ([inventory.ts:341](/Users/peterovchinnikov/work/awesome/true-bdd/harness/app/lib/view-model/inventory.ts:341)) but is never passed by the page.

Because `selected` remains set, the modal can also reopen unexpectedly if that identity later reappears.

Concrete fix: retain the last modal presentation only as a disappearance fallback, render it with `changedOnDisk: true`, and keep current objects for all normal updates. Add a browser test that removes the selected story, promotes a new generation, and asserts the dialog remains open with `data-reason="changed_on_disk"`.

## NICE-TO-HAVE

- Complete the specified display fallback chain. `storyModalModel` always uses `create_id` and allows an empty title ([inventory.ts:353](/Users/peterovchinnikov/work/awesome/true-bdd/harness/app/lib/view-model/inventory.ts:353)); the component then bypasses `model.storyId` entirely ([StoryModal.tsx:80](/Users/peterovchinnikov/work/awesome/true-bdd/harness/app/components/StoryModal.tsx:80)). Use file-content ID/title → declared ID/title → create identity/`unknown` for the heading, while retaining `data-story-id=create_id` for selector compatibility.

- Arrow-key tab selection does not move focus to the newly selected tab ([StoryModal.tsx:48](/Users/peterovchinnikov/work/awesome/true-bdd/harness/app/components/StoryModal.tsx:48)). Implement roving focus and assert the destination tab is focused, not merely `aria-selected`.

## Verification performed

- `go test ./...` — passed.
- `npm run typecheck` — passed.
- `npm run test:unit` — 164/165 passed. The sole failure was unrelated `start-identity.test.ts`: this sandbox forbids `ps`, confirmed directly as `operation not permitted`.
- Rework-specific Vitest files — 28/28 passed.
- `git diff --check` — passed.
- AI-spec diff — empty.
- Origin-policy diff — empty; the origin/host gate still executes before body reading.
- Playwright and real-Claude/AI operations were not run, as instructed; recorded protocol result remains 33/33.