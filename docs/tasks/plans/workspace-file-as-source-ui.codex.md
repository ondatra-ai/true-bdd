# Codex rounds ledger — workspace-file-as-source-ui

Scoring is the AGENT's (codex-loop.md): keep a finding only if composite ≥7 AND
all four gates pass (Correctness, Evidence, Scope fit, Regression risk). Codex is
read-only and never edits. Hard-lane cap: ≤3 rounds; stop early on a dry round.

Plan under review: `docs/tasks/plans/workspace-file-as-source-ui.md`.

---

## Round 1 of 3

- Prompt: `./tmp/codex-workspace-plan-r1.md` · Response: `./tmp/codex-workspace-plan.md` (14 findings)
- Kept: 14 / 14 · Skipped: 0. (Unusually strong round — evidence-grounded in
  `docker-compose.test.yml`, `remote-process.ts`, `wire.go`, `handlers.go`,
  `playwright.config.ts`, `clickup-reference.md`, and cited plan lines.)

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regress | Keep | Applied |
|---|---|---|---|---|---|---|---|---|
| 1 | S1 proof overclaims uniqueness (container→host, not CLI) | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | Target-state + Challenges: two-part S1 oracle (unmounted container + CLI write receipt correlated across browser resp + `/api/agent/reply`); w4.1 rewritten. |
| 2 | Real chat integration (a10) architecturally unspecified | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | Added "Chat transport" (chat work item → Claude turn → structured result → buffer → doc_write); Slice 5. |
| 3 | P18a has no gating coverage | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | Added w5.1a (four groups/order, fixture counts, header/feature/story/scenario nav + jumps, live add/remove). |
| 4 | Several requirements ungated (S2,P9,P10-Home,P15,P16,P18,P21,P22,P23) | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | S2 note (w1.1); w3.1 tech stack; w3.4 edit endpoints; w5.1 PRD+features pages; w5.4 description+live; w5.5 existing-pick; w6.3 Home; w6.5 product live-derive. |
| 5 | P7 tests token plumbing not ClickUp parity | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Added w7.1a parity matrix + uncovered testable behaviors (rest/hover caret, glyph dir, guide line, bottom-pinned utilities, chat input pinned, wide default). |
| 6 | Writes lack atomicity/ordering/stale protection | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | doc_write → `{path,content,base_revision,client_token}`; per-doc serialize + stale conflict + atomic temp+fsync+rename + idempotent replay; Slice 0 Go tests. |
| 7 | Plan resolved chat-vs-typing open question by fiat | 9 | ✅ | ✅ | ✅ | ✅ | KEEP | Removed "default last-writer-wins"; now detection-mechanism-only (revision mismatch → neutral conflict), policy left undecided (derivation model + Challenges). |
| 8 | Assertions pass with placeholder/wrong impls | 9 | ✅ | ✅ | ✅ | ✅ | KEEP | Anti-placeholder discipline block (unique per-run values, parse+exact-node, snapshot+intercept, descendant-of-correct-row); fixed w5.7 spanning-regex bug; w2.1/w3.4/w6.3 hardened. |
| 9 | Restart/persistence cases order-dependent | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | w4 note: atomic w4.1–w4.3 or per-case seeds; restart waits for re-registration. |
| 10 | Flake mitigations incomplete | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | Observable save-state for negatives; mid-file jump target + clamp assert; bbox pointer for hover/resize; Challenges flake list. |
| 11 | Negative-S1 "session drop" underspecified | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | w4.5 uses SESSION_GONE helper + exact code + bounded deadline + snapshot; added agent-dies-after-accept case. |
| 12 | Path safety / story-create semantics underspecified | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | doc_write path policy (relative-only, canonical containment, symlink resolve, .yaml, glob allowlist, exclusive-create); Slice 0 Go tests. |
| 13 | Sequence not red→green at requirement granularity | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Implementation reframed as vertical Slices 0–5, each greening a w-cluster; tests-first preserved. |
| 14 | P20 "exactly id+description" only presentation-enforced | 7 | ✅ | ✅ | ✅ | ✅ | KEEP (narrowed) | w5.3/w5.5 parse features.yaml + assert exact key set on seeded + created stub; free-text stays YAML-gated only (respects P9); boundary noted in Challenges. |

Round 1 was highly productive (14/14 kept) → proceed to round 2 (verify
applications + challenge nothing-skipped + fresh findings).

---

## Round 2 of 3

- Prompt: `./tmp/codex-workspace-plan-r2.md` · Response: `./tmp/codex-workspace-plan.md` (21 items)
- Verification of round-1 applications: **RESOLVED** #7, #9, #10, #13, #14 (no
  action); **PARTIAL** #1, #2, #3, #4, #5, #6, #8, #11, #12 (refined below). Item
  15 confirms no round-1 finding should have been skipped. Fresh findings #16–#21.
- Kept: **15** (9 partial-refinements + 6 fresh) · Skipped: 0. Strong feasibility
  catches grounded in `scheduler.go`, `wire.go`, `handlers.go`, `client.go`.

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regress | Keep | Applied |
|---|---|---|---|---|---|---|---|---|
| 1 | `/api/agent/reply` is CLI→relay; browser can't intercept it | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | S1 oracle now: browser doc_write RESPONSE receipt + a test-only relay receipt-audit hook keyed by work_id; receipt oracle also on chat (w6.3/a10). |
| 2 | Chat structured-result has no DTO/patch/validation/failure | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | Chat DTOs specified: `{conversation,current_path,current_content}`→`{reply_text,edit|null}`, FULL-CONTENT (no patch), target-binding, malformed/YAML-invalid/timeout branches, write scheduler class; Go tests; Slice 5. |
| 3 | "typing a new story file" not executable; no manifest refresh | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | w5.1a live-add now via P22 create-UI + provider re-fetch of `doc_tree`; within-file typing moved to the scenarios case; derivation model notes cross-file refetch. |
| 4 | Remaining gaps: P1 Home/Builds, P9 PRD/features edit, P21 by-hand, P10 leaves | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | Added w1.1b (Home+Builds dock/active/URL/landing); w5.4 by-hand rebucket; w5.4a direct PRD/features edit+persist. |
| 5 | Parity matrix still misses measured behaviors | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | w7.1a adds flyout-float-right + chat header; classifies quick-actions + floating edit toolbar as adapted/deferred (nothing left unmapped). |
| 6 | revision-as-counter unsafe; scheduler maps unknown→ClassRead | 9 | ✅ | ✅ | ✅ | ✅ | KEEP | revision = SHA-256(bytes)+existence recomputed under lock; Slice 0 must classify doc_read/doc_write/chat scheduler classes; stale-after-restart/external-edit tests. |
| 8 | "every case" overclaims; intercept not conclusive | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Anti-placeholder block scoped (unique-token → content-echo cases); full-tree byte snapshot is the AUTHORITATIVE negative oracle, intercept diagnostic. |
| 11 | w4.5 doesn't name status/body; after-accept result missing | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | w4.5 names 404 session_gone / 504 cli_timeout + no false `saved`; after-accept may commit CLI-side, assert no false success. |
| 12 | Path namespace inconsistent (allowlist vs real subdirs) | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | Allowlist = exact patterns (docs/architecture/architecture.yaml, docs/prd/prd.yaml, docs/prd/features.yaml, docs/scenarios.yaml, docs/prd/stories/*.yaml); final-target symlink + wrong-dir reject tests. |
| 16 | Story create not atomic across two files (fresh) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP (narrowed) | Ordering feature-stub-first-then-story so partial failure is benign (unreferenced feature); batch-write noted as future, not built (avoids over-scope). |
| 17 | Idempotency lifetime/token-conflict underspecified (fresh) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | client_token keyed by request-hash{path,content_hash,base_revision}; identical→original receipt, differing→409; one-agent-lifetime. |
| 18 | Atomic-write durability incomplete (fresh) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Commit now: temp safe perms → file fsync → rename → parent-dir fsync → mode preservation → temp cleanup; failure-injection tests. |
| 19 | w5.5 can pass without proving SEARCHABLE (fresh) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | w5.5(a) types a filtering query, asserts nonmatches absent, then picks survivor. |
| 20 | Service sub-hierarchy effectively chosen (fresh) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | w3.1: sidebar stays one-entry-per-service (open question); tech stack/endpoints asserted in the file page's per-service details region, not nested sidebar. |
| 21 | Builds placeholder vs "hosts runs pages" inconsistency (fresh) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Builds = navigation-only landing (no editor/chat target); gated in w1.1b; runs pages not rebuilt. |

RESOLVED (no action): #7 (open-question fiat removed), #9 (order-independence),
#10 (flake mitigations), #13 (red-first + slices), #14 (P20 parsed keys).

Round 2 was productive (15 keeps, incl. feasibility catches) → run round 3 (final,
cap reached) to verify these refinements and catch any residual.

---

## Round 3 of 3 (FINAL — cap reached)

- Prompt: `./tmp/codex-workspace-plan-r3.md` · Response: `./tmp/codex-workspace-plan.md` (17 items)
- Verification of round-2 applications: **RESOLVED** #3, #4, #5, #8, #11, #12,
  #16, #17, #19, #20, #21 (11 items, no action); **PARTIAL** #1, #2, #6, #18 (4
  incomplete applications, refined below). Item 16 confirms no round-2 skip. No
  materially-important fresh finding. Verdict: sound after these 4 corrections.
- Kept: **4** (the PARTIAL refinements) · Skipped: 0.

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regress | Keep | Applied |
|---|---|---|---|---|---|---|---|---|
| 1 | w4.1 still cited `/api/agent/reply` (browser can't see it); audit seam unscoped | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | w4.1 reworded to browser-response receipt + test-only audit hook; Slice 0 relay scopes the receipt-audit seam (Redis, TTL, test-gated route) + helper reader. |
| 2 | Chat Go-test list omits timeout/cancellation branch (both lists) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Added timeout/cancellation (+ expected status) to the Target-state and Slice 5 chat Go-test lists. |
| 6 | No generic write lane in `pool.go`; "write class" ambiguous; slow chat could stall runs | 9 | ✅ | ✅ | ✅ | ✅ | KEEP | Specified: add `ClassDoc` + a SEPARATE chat lane (not `ClassDispatch`) so a slow chat can't occupy the run mutation slot; ordering tests incl. slow chat vs run dispatch. |
| 18 | Durable-commit ordering ambiguous (mode preservation placement) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Exact order: temp with intended mode up front (preserve on update / 0644 on create) → write → file fsync → rename → parent-dir fsync → temp cleanup. |

RESOLVED (no action): #3, #4, #5, #8, #11, #12, #16, #17, #19, #20, #21.

**Loop closed at the hard-lane 3-round cap.** Totals: R1 14/14, R2 15/15, R3 4/4 —
44 findings kept, 0 skipped (no finding failed the gates across all three rounds).
Every round-N application was verified in round N+1; the final 4 partials are
applied. Codex's closing verdict's four blockers (receipt-audit seam, chat timeout
test, scheduler-class design, durable-write ordering) are all addressed. Plan is
ready for the test-author.

---

# Test-author Codex rounds (deliverables: workspace e2e tests + startup scaffolding)

Separate loop from the planner's rounds above — the artifact under review is the
TEST-AUTHOR's output (the `w*`/a10 specs, `workspace-env.ts`, `ui.ts`/README
additions, the `w-workspace-happy` fixture, and the behavior-free `harness/app/`
scaffolding), not the plan. Scoring is the AGENT's (composite ≥7 AND all four
gates). Hard-lane cap ≤3.

## Round 1 of 3

- Prompt: `./tmp/codex-workspace-tests-r1.md` · Response: `./tmp/codex-workspace-tests-r1.md`
  (label collision overwrote the prompt with the 11-item answer — fixed labeling
  for r2). Codex verified: all fixture YAML valid + structurally aligned; harness
  strict `tsc` passed; no Playwright project-routing leak (`w*`→workspace, a10→ai).
- Kept: **10 / 11** · Skipped: **1**.

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regress | Keep | Applied |
|---|---|---|---|---|---|---|---|---|
| 1 | Snapshot omits `docs/prd/stories/*.yaml`; negative oracles miss story write/create | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | `workspace-env`: `FIXED_DOCS`+`allowedDocPaths()` (dynamic story files); `snapshotDocs`/`changedDocsSince` compare the UNION so a NEW story file is detected. |
| 2 | a10 never reads the relay receipt-audit hook (not the two-part proof) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | a10 now reads the audit hook + correlates by `work_id`. |
| 3 | Receipt correlated by path/hash, not `work_id` (`work_id` optional) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | `WriteReceipt.work_id` REQUIRED; w4.1/w6.3/a10 match browser↔audit by exact `work_id`; README updated. |
| 4 | w6.3 appends a comment only; no live-outline assertion | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Deterministic driver `@probe add-term`; w6.3 asserts the Terms outline row appears live + parses the term node. |
| 5 | w6.3 non-file check waits only for history token (delayed edit could slip) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Waits for a new assistant message (turn finished) BEFORE the snapshot. |
| 6 | w4.5a accepts any 404/504 body (not the exact mapped error) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Asserts `body.error` == `session_gone` (404) / `cli_timeout` (504). |
| 7 | w4.5b kills the agent before the debounced write is even sent | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | `waitForRequest(/docs/write/)` BEFORE `killAgent` — die after accept. |
| 8 | w5.3 asserts only seeded disk files; passes with no P20 UI behavior | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Parses the workspace-SERVED editor buffers (features/story/scenarios pages). |
| 9 | w5.4/w5.7 reassign to FIXED fixture strings (plan wants unique per-run) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | `seedFeatureOnDisk()` unique target; node-scoped parsed disk assertions. |
| 10 | Positive YAML oracles use substring, not node-scoped parse | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | w4.1/a10 parse the committed file and assert the term under the `terms` node. |
| 11 | `tests/harness/package.json` yaml devDep is outside the listed deliverables | 5 | ❌(fix) | ✅ | ❌ | ❌(fix) | SKIP | The plan MANDATES a real YAML parser ("do not reuse the prototype's line-heuristic regex"); Node ships none; the change is confined to the test-author-owned e2e tree (`tests/`, NOT production). The proposed "revert to a hand-rolled parser" fails Correctness/Regression (increases oracle fragility). Disclosed in the report instead. |

Round 1 was highly productive (10/11 applied) → run round 2 to verify applications
+ challenge the skip + catch residual.

## Round 2 of 3

- Prompt: `./tmp/codex-workspace-tests-r2.md` · Response: `./tmp/codex-workspace-tests.md` (5 items)
- Verification of round-1 applications: #1,#2,#4–#10 confirmed applied; #3 and the
  chat/negative cases were flagged as INCOMPLETE (refined below). The SKIP (#11)
  was challenged: Codex agrees a real parser is required and a hand-rolled one
  would weaken the oracle — so the skip's technical reason HOLDS; its remedy is
  disclosure/authorization, which the report does.
- Kept: **4** · Skipped: **1** (the re-raised yaml-dep scope point → disclose).

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regress | Keep | Applied |
|---|---|---|---|---|---|---|---|---|
| 1 | w6.3/a10 correlate by work_id but compare only content_hash (not path/revision) | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | w6.3 + a10 now assert browser `path` and audit `{path, committed_revision, content_hash}` — mirror w4.1. |
| 2 | `gotoWorkspace` = `page.goto` (full nav) destroys persistent-layout state; w6.2/w6.3 test PRESERVATION | 9 | ✅ | ✅ | ✅ | ✅ | KEEP | w6.2 navigates via rail+sidebar Links; w6.3 non-file loop navigates client-side (rail→feature row / rail-home). (w1.5 already used rail clicks; w1.6 is URL-derived highlight, not persistent state — untouched.) |
| 3 | w5.4 by-hand re-bucket verified via a full-reload `gotoWorkspace` (not "live without reload") | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | w5.4 navigates to the story page AND back to the aggregation via client-side rail+sidebar Links. |
| 4 | w6.5 chat uses `@probe append` (comment); no scenarios-outline change asserted | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | w6.5 chat uses `@probe add-scenario <ID>`; asserts the live `scenario-row` + parses the scenario node. |
| 5 | `tests/harness/package.json`/lockfile yaml dep outside enumerated deliverables | 5 | ✅(reason holds) | ✅ | ❌(fix) | ❌(fix) | SKIP | Codex confirms a real parser is required; the remedy is authorization/disclosure, not a code change. Disclosed prominently in the test-author report + here. |

Round 2 applied 4 (incl. the important `page.goto`-defeats-persistence catch) → run
round 3 (final, cap) to verify these applications and catch residual.

## Round 3 of 3 (FINAL — cap reached)

- Prompt: `./tmp/codex-workspace-tests-r3.md` · Response: `./tmp/codex-workspace-tests-final.md` (4 items)
- Verification of round-2 applications: #1–#4 confirmed correctly applied. Three
  RESIDUAL rigor gaps surfaced (below) + one report-framing correction.
- Kept: **3** · Skipped/framing: **1** (the yaml dep — now marked APPLIED+disclosed, not a reverted skip).

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regress | Keep | Applied |
|---|---|---|---|---|---|---|---|---|
| 1 | w4.5b still permits a FALSE-success `200 {}` (receipt optional) | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | On 200, REQUIRE a complete receipt `{path, content_hash, work_id}`, correlate the audit record by `work_id`, assert `{path, committed_revision, content_hash}` vs disk. |
| 2 | w6.5 by-hand + chat ids share a range → collision pre-satisfies the chat half | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Disjoint id ranges (`E2E-1xx` vs `E2E-8xx`); install a doc-write spy; require the CHAT'S OWN new doc_write whose receipt matches disk. |
| 3 | w5.5 "feature required" negative oracle only counts sidebar rows | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Snapshot before the featureless submit; assert `changedDocsSince` empty (authoritative); assert the form stays open. |
| 4 | Report framing: the yaml dep is APPLIED, not a reverted skip | — | ✅ | ✅ | n/a | n/a | ACK | The `yaml` devDep in `tests/harness/package.json` is a deliberate, correct, DISCLOSED test-author addition (plan mandates a real parser; Node ships none). Framed as applied+disclosed, not skipped. |

**Loop closed at the hard-lane 3-round cap.** Totals (test-author): R1 10/11, R2 4/5,
R3 3/4 — 17 code findings applied, 0 unresolved. The one non-code item (yaml
devDep) is a necessary, disclosed test dependency (Codex confirmed a real parser is
required and a hand-rolled one would weaken the oracles). Every round-N application
was verified in round N+1; the RED baseline was reconfirmed after the edits (w4.1 +
w6.3 still fail at the `GET /api/sessions` 501 session-appear gate). Tests are RED,
rigorous (parsed node-scoped oracles, two-part work_id-correlated S1 receipt,
client-side nav for persistence tests, authoritative byte-snapshot negatives),
and ready for the coder.

---

# Reviewer Codex rounds (final review — production diff + e2e/unit tests)

Separate loop from the planner/test-author rounds above — the artifact is the
COMPLETED, green implementation (production Go + `harness/app/` + the e2e/unit
tests). Scoring is the AGENT's (composite ≥7 AND all four gates). Hard-lane cap ≤3.

## Round 1 of 3

- Prompt: `./tmp/codex-wsreview-r1.md` · Response: `./tmp/codex-wsreview.md` (5 findings).
  Codex verified claims by reading `docs.go`, `docs_internal_test.go`, `w4-persistence.spec.ts`,
  `files-context.tsx` with `nl -ba`/`rg`.
- Kept: **2** · Skipped: **3**.

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regress | Keep | Disposition |
|---|---|---|---|---|---|---|---|---|
| 1 | doc_write path-validation TOCTOU (parent-dir symlink swap between validate and open) | 5 | ~ | ✅ | ❌ | ❌ | SKIP | Out of the plan's stated path-safety scope (submitted-path symlink resolve + containment — implemented). Exploit needs a concurrent local attacker who already has project write access (no privilege gain); the fix is a Linux-only `openat2`/BENEATH rewrite (high regress risk, non-portable on macOS). Residual risk noted. |
| 2 | Concurrent idempotency: two same-token requests can both miss dedup / both commit | 6 | ✅ | ✅ | ✅ | ❌ | SKIP | No manifestation under the real client contract (a FRESH randomToken() per commit; never concurrently reuses a token across paths). The per-document lock already prevents disk corruption/lost-update; worst case is a benign surfaced `conflict`. Atomic-reservation rework carries regress risk on green code. Residual robustness noted. |
| 3 | doc_write pre-read conflates ALL read errors with "absent" → can clobber an unreadable existing file | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Applied: `currentState()` treats only `errors.Is(readErr, os.ErrNotExist)` as absent; any other read error → `docWriteIOError`. Added `TestDocWriteRejectsUnreadableExistingTargetInsteadOfClobbering` (chmod 000, root/ACL-skip guard). read()'s unreadable-file display left as-is (error-states-beyond-invalid-YAML is an explicit brief open question). |
| 4 | Parent-dir fsync failure discarded while receipt reports durable | 5 | ❌(fix) | ✅ | ❌ | ❌ | SKIP | Deliberate, code-documented design: `rename` IS the atomic commit; parent-dir fsync is best-effort crash-durability polish. Codex's fix (fail the write after a successful rename) would report FALSE FAILURE for an already-committed write — a correctness regression. Intentional; noted. |
| 5 | w4.4 invalid-YAML negative oracle can't isolate the client-side gate from the CLI gate | 7 | ✅ | ✅ | ✅ | ✅ | KEEP | Applied: `installDocWriteSpy` before breaking YAML; assert `spy.writeCount` unchanged (the client issued NO doc_write for the invalid buffer) alongside the existing byte-snapshot + last-valid oracles. Sleep-free (rides the observable invalid save-state). |

Two keeps → run round 2 (verify F3/F5 applications, challenge the F1/F2/F4 skips, fresh findings).

## Round 2 of 3

- Prompt: `./tmp/codex-wsreview-r2.md` · Response: `./tmp/codex-wsreview2.md` (6 items).
- Verification of round-1 applications: **A3 RESOLVED** (io_error guard confirmed).
  **A5** flagged NOT-RESOLVED — but on a MISREAD (see below). Skips **S1/S2/S4 all
  UPHELD** by Codex under the stated threat model. One FRESH keep (#6).
- Kept: **1 fresh (#6)** + **1 hardening (A5 root)** · Skipped: 0 new · Skips upheld: 3.

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regress | Keep | Disposition |
|---|---|---|---|---|---|---|---|---|
| A5 | w4.4 client-gate isolation "NOT-RESOLVED": a valid save pending, then invalid edit within debounce, could fire commitSave with the invalid buffer | 7 | ✅(root) | ✅ | ✅ | ✅ | KEEP(root) | Codex MISREAD `scheduleSave`: `clearTimeout(existing)` is UNCONDITIONAL at files-context.tsx:214-217, BEFORE the invalid check — an invalid edit DOES cancel a pending save, so the imagined stale-fire does not occur. But hardened the ROOT anyway: `commitSave` now re-validates `isValidYAML(doc.buffer)` before writing (defensive belt independent of timer lifecycle) → surfaces `invalid`, never commits. |
| 6 | CLI YAML gate accepts multi-document streams the browser's strict single-doc `YAML.parse()` rejects | 8 | ✅ | ✅ | ✅ | ✅ | KEEP | Confirmed: browser `isValidYAML` (yaml-utils.ts) uses `yaml.parse()` which throws on multi-doc; Go `isValidYAML` used `yaml.Unmarshal` (decodes first doc only). A direct doc_write of `a\n---\nb` would commit content the workspace treats as invalid — the codebase already documents this mismatch in chat.go. Fix: `isValidYAML` now uses `yaml.NewDecoder` and rejects a second document (empty/comment-only and leading-`---` single docs stay valid). Added `TestDocWriteRejectsMultiDocumentStream` + `TestIsValidYAMLSingleDocumentAndEmptyStillValid`. |

RESOLVED (no action): A3. UPHELD skips (no action): S1 (TOCTOU needs a local FS race, not a
browser payload), S2 (no client token-reuse / lost update / false success — fresh token per
commit + per-doc lock), S4 (rename precedes dir-sync; ignoring dir-sync can't report a false
success, only leaves crash-durability uncertain).

Round 2 produced a fresh keep (#6) + a root hardening (A5) → run round 3 (final) to verify.

## Round 3 of 3 (FINAL — cap reached)

- Prompt: `./tmp/codex-wsreview-r3.md` · Response: `./tmp/codex-wsreview3.md` (4 items).
- Verification: **#6 RESOLVED** (Codex ran the installed browser `yaml.parse()` and confirmed
  the CLI gate now MATCHES it: empty/comment-only/leading-`---` valid, populated AND empty
  trailing docs rejected). **A5 RESOLVED** (commitSave fire-time guard + confirmed
  scheduleSave's unconditional clearTimeout — the round-2 misread corrected). **S1/S2/S4
  skips remain correctly upheld.** No additional material production/test defect.
- Kept: **0** (DRY round) · Skipped: 0.

**Reviewer loop closed at the hard-lane 3-round cap — final round DRY.** Totals: R1 2 keeps /
3 skips, R2 2 keeps (1 fresh #6 + 1 root A5) / 3 skips upheld, R3 0 keeps. **4 reviewer
findings applied**, 0 unresolved:
1. `docs.go` — write path refuses a non-ErrNotExist pre-read (io_error) instead of clobbering
   an unreadable existing file (+ test).
2. `w4-persistence.spec.ts` — invalid-YAML negative oracle installs a doc-write spy and asserts
   zero client writes for the invalid buffer (isolates the client gate from the CLI gate).
3. `docs.go` — `isValidYAML` rejects multi-document YAML streams, aligning the authoritative
   CLI gate with the browser's strict single-document `yaml.parse()` (+ 2 tests).
4. `files-context.tsx` — `commitSave` re-validates the buffer at fire time so a debounced
   autosave never commits invalid content, independent of the timer lifecycle.
Every round-N application was verified in round N+1; all three skips (path TOCTOU, concurrent
idempotency, dir-fsync-on-failure) were challenged and upheld within the stated threat model.
Post-hardening gates: go build/test OK, golangci-lint 0 issues, harness tsc + eslint clean,
vitest 25/25.
