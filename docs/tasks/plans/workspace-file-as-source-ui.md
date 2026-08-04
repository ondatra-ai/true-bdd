# Plan — workspace-file-as-source-ui

Tests-first plan for `docs/tasks/workspace-file-as-source-ui.md` (hard lane).
Paths taken from `docs/context/paths.md`. Lead with the e2e layer, then the
production changes it drives.

The brief carries the validated requirement set (S1–S2, P1–P25 + sub-items), a
terminology note (a *scenario* IS the requirement entity; "Requirements" lists
scenarios), explicit **open questions** (planned around, never resolved by fiat),
and an **Established facts** appendix this plan TRUSTS verbatim.

---

## Goal

Ship the production TrueBDD harness workspace as a ClickUp-patterned,
file-as-source web app: a fixed-schema navigation over the host project's real
YAML documents (architecture spec + product docs), edited seamlessly in place or
through a docked chat, with **every committed change persisted through the
true-bdd CLI** (S1), and features as the tag aligning stories and scenarios.

## Non-goals

- The full v2 relay run/prompt/build machinery (sessions runs, prompt dialogs,
  build-tests/build-code, inventory scan). Those belong to the `p*`/`a*`
  relay contract, currently unimplemented after the harness gut (commit
  ba52d00). This task builds ONLY the relay surface the workspace needs
  (agent register/poll/reply + session listing + the new **document** work
  items) and the workspace UI on top.
- Builds section as file-as-source: the brief scopes Builds to **navigation
  level only** (a rail section that will host the runs pages later). This task
  ships the rail entry + an empty/placeholder Builds landing, nothing more.
- Resolving any **open question** (see that section). The plan builds the
  *mechanism* each open question sits on and leaves the *policy* to review.
- docker-compose generation from the spec; prd `stories:` index auto-sync;
  service outline sub-hierarchy; error/empty states beyond invalid-YAML — all
  explicitly deferred by the brief.
- Copying the prototype (`harness/design/proto-workspace/`) wholesale. It is a
  design reference (in-memory, no persistence, line-heuristic parsing); production
  replaces its store with CLI persistence and its heuristics with a real YAML
  parser.

## Current state

- `harness/` is **greenfield**: only config (`next.config.ts` standalone,
  `package.json` with `dev -p 4517`, `Dockerfile`, `vitest.config.ts`,
  `tsconfig`, eslint). **No `app/`, no pages, no API routes.** `next build`
  today produces no servable app; commit ba52d00 gutted the harness source.
- The CLI (`src/`) runs `true-bdd remote` (`src/cmd/remote.go` →
  `src/internal/app/remote/`): registers to the harness, polls for work items,
  replies. Work item types today: `query` (views `session_status`,
  `session_detail`, `run_detail`), `dispatch` (run a command), `answer`
  (`src/internal/app/remote/wire.go`). It already reads/scans host files and
  already writes YAML (`us apply` merges `docs/scenarios.yaml`). No document
  read/write work items exist.
- E2e infra is ready-made and exactly the shape this task needs:
  `tests/harness/` is a self-contained Playwright package; `ServerController`
  launches one harness **container** per test (`docker-compose.test.yml`, its
  own port, **no host-folder mount**); `RemoteProcess` spawns `true-bdd remote`
  with cwd = a materialized fixture folder; `ProtocolEnv.start/materialize/
  startRemote` wire container + agent + Redis + fixture together;
  `global-setup` builds the image + Go binary + Redis once. Projects route by
  filename: `p*`→protocol (no Claude), `a*`→ai (real Claude), `m*`→mockups.
- Design reference only: 17 static mockups (`harness/design/mockups/`, m1–m6
  suite) and the working prototype (`harness/design/proto-workspace/`) with
  `clickup-reference.md`. Design system mirror at `harness/design/system/`
  (`tokens.css`): square corners, no drop shadows, Poppins-only — file views
  need a **scoped monospace exception**.

## Target state

A session-scoped workspace served by the harness container, backed by the CLI.

**Routing (session-scoped, consistent with the existing session model).** The
workspace edits ONE host project = one connected CLI agent = one session. A
persistent route-group layout survives client-side navigation (Established
fact: components in a persistent Next layout keep state across nav):

- `/` — sessions list (minimal: lists connected folders so a session id resolves).
- `/sessions/<sid>/(workspace)/home` — workspace overview (no single backing file).
- `/sessions/<sid>/(workspace)/architecture` — architecture.yaml file page.
- `/sessions/<sid>/(workspace)/product` — PRD (prd.yaml) file page = Product root.
- `/sessions/<sid>/(workspace)/product/features` — features.yaml file page.
- `/sessions/<sid>/(workspace)/product/features/<id>` — feature aggregation (derived, no file).
- `/sessions/<sid>/(workspace)/product/stories/<storyId>` — story file page.
- `/sessions/<sid>/(workspace)/product/scenarios` — scenarios.yaml file page.
- `/sessions/<sid>/(workspace)/builds` — navigation-only landing.

**Persistence architecture (the cross-service decision S1 demands — RECOMMENDED,
flagged for review/Codex ratification).** Extend the **existing relay agent
protocol** with three new **synchronous document work items** handled by the
CLI's poll handler (parallel to `query`, off the run machinery):

- `doc_tree` — returns the fixed-schema document manifest: which real file paths
  back which navigation nodes, each with its current `revision`. `revision` is a
  **content-derived** value — `SHA-256(bytes)` + existence state, recomputed under
  the per-document lock (NOT an in-memory counter, which can't detect a stale write
  after CLI restart or an external host edit — r2 #6).
- `doc_read {path}` — returns the file's current bytes + existence/parse status +
  `revision`.
- `doc_write {path, content, base_revision, client_token}` — a **revisioned,
  serialized, atomic** write and the S1 persistence backend. The CLI:
  (a) **path-safety** (r1 #12 / r2 #12) — workspace-relative paths only, rejecting
  absolute/`..`-traversal/NUL/non-`.yaml`, resolving the **final target** symlink
  (not just parents) and enforcing canonical containment. The allowlist is an
  **exact-pattern set** — `docs/architecture/architecture.yaml`, `docs/prd/prd.yaml`,
  `docs/prd/features.yaml`, `docs/scenarios.yaml`, `docs/prd/stories/*.yaml` — so a
  similarly-named file in the wrong directory is rejected, and P22 can
  exclusive-create a *new* story file while everything else is update-only;
  (b) **YAML gate** — refuses content that does not parse (P10a/P10b);
  (c) **concurrency** — serializes writes per canonical document and rejects a
  **stale `base_revision`** (revision recomputed under the lock immediately before
  commit) with a neutral `conflict` result (mechanism only — the resolution
  *policy* is an open question); (d) **atomic + durable commit** in this exact
  order (r2 #18 / r3 #18): create a same-dir temp file **with the intended mode
  applied up front** (update → preserve the existing file's mode; create → `0644`)
  → write bytes → **file fsync** → atomic **rename** over the target →
  **parent-dir fsync** → clean up any abandoned temp; (e) returns a **write
  receipt** `{path, committed_revision, content_hash, bytes}`.
- **Idempotency (r2 #17):** `client_token` dedup is keyed by a request hash over
  `{canonical_path, content_hash, base_revision}` (mirroring the existing dispatch
  arg-fingerprint in `handlers.go`): an identical replay returns the original
  receipt; the SAME token reused with DIFFERENT args → `409 conflict`. Dedup is
  bounded to one agent lifetime (in-memory; a restart clears it, matching the
  ephemeral-session model).
- **Scheduler classes (r2 #6 / r3 #6):** `scheduler.go` defaults unknown work to
  `ClassRead` and `pool.go` has only `ClassAnswer`/`ClassDispatch`/`ClassRead`/
  `ClassInventory` — there is no generic write lane. Slice 0 adds a **distinct
  `ClassDoc`** for `doc_read`/`doc_tree`/`doc_write` (its own concurrency lane) and
  runs the **chat** item in a **separate lane** (not `ClassDispatch`), so a slow
  Claude chat turn CANNOT occupy the run mutation slot and stall `dispatch`/
  `answer` (honouring the "don't affect run machinery" guard). Ordering tests cover
  `answer` / `dispatch` / `doc_write` / chat / reads, incl. a deliberately slow
  chat turn not blocking a concurrent run dispatch.
- **Story creation ordering (r2 #16):** P22's create-new touches two files
  (features.yaml stub + the new story file) via two `doc_write`s. Rather than a
  full transaction, order them **feature-stub-first, then the story file**, so any
  partial failure leaves at most a *benign unreferenced feature* (an empty
  aggregation, valid per P21) — never a story pointing at a missing feature. A
  transactional `doc_batch_write` is noted as a future option, not built here
  (avoids over-scope; mechanism-only, no policy fiat).

Browser ↔ relay is session-scoped: `GET /api/sessions/:sid/docs` (tree),
`GET …/docs/read?path=`, `POST …/docs/write {path,content,base_revision,client_token}`.
Each route turns into a document work item the relay carries to the CLI; the relay
never touches the filesystem. `doc_write` is a **mutation** → exact Origin+Host
enforced before business logic (existing policy).

**Why this architecture (and how S1 is actually proven — r1 #1).** The whole
harness is a stateless relay; the CLI connects OUT (works with a deployed Vercel
harness where inbound to the host is impossible); the e2e infra already launches
container + agent + Redis. **`docker-compose.test.yml` gives the container no
mount of the host project folder** (no `volumes:`), so the Next relay demonstrably
*cannot* write the project files — that rules out the relay. But the unmounted
container alone only proves "a host-side process wrote it" (Playwright shares the
host fixture dir); it does not by itself prove *the CLI* did. The S1 oracle is
therefore **two-part**: (1) the on-disk change (relay ruled out), AND (2) the
**CLI write receipt**. `/api/agent/reply` is CLI→relay traffic a browser
interceptor cannot see (r2 #1), so the receipt is verified two ways: the browser
`doc_write` **response** carries the receipt the relay propagated from the CLI
reply, AND a **test-only relay receipt-audit hook keyed by `work_id`** (readable by
the test helper) records the CLI-originated receipt; both must carry
`{path, committed_revision, content_hash}` matching the on-disk `SHA-256`. Together
they prove browser → relay → **CLI** → disk. The same receipt oracle is applied to
at least one **chat** edit (w6.3 / a10) so S1 is proven for chat, not just direct
edits.
The alternative (harness bind-mounts the folder + shells a `true-bdd docs`
subcommand) is weaker still: the Next server could write directly, so even the
receipt would not exclude a direct write, and it breaks the deployed-relay model.
See Challenges.

**Chat transport (production, r1 #2 / r2 #2) — the same persistence path.** Chat is
a new narrowly-scoped **chat work item** (NOT the run/build machinery). Request DTO:
`{conversation, current_path|null, current_content|null}`. The CLI runs one Claude
turn (reusing `src/claudecode/`'s streaming client) and returns a **structured
result** DTO `{reply_text, edit: {path, new_content}|null}` — deliberately
**full-content replacement** (no patch grammar to parse/validate), so the edit
flows through the SAME `doc_write` YAML gate and revision mechanism. Rules: the
`edit.path` MUST equal `current_path` (target binding); a malformed/absent
structured result → the turn returns `reply_text` with `edit=null` and a surfaced
error (never a blind write); YAML-invalid `new_content` is refused by `doc_write`
like any edit; the turn honours a timeout/cancellation and is scheduled in the
write class. The browser applies `edit.new_content` to the editing **buffer**
(reflects immediately, P10) and the normal debounced **`doc_write`** persists it —
chat and direct edits share ONE persistence path (S1) and one conflict/revision
mechanism. On non-file pages (Home, feature aggregation) `current_path` is null, so
the result has `edit=null` and no `doc_write` is issued (P10). The **deterministic
chat driver** (Challenges) short-circuits the Claude turn with a scripted
structured result so protocol tests exercise the identical buffer→doc_write path
without a model call; `a10` proves the real Claude turn. Go tests cover the
malformed-result, wrong-target, YAML-invalid, and **timeout/cancellation**
branches (each with its expected surfaced result/status) before `a10`.

**Derivation model (client-side; CLI is persistence only).** The FilesProvider
(session-scoped, in the persistent layout) loads the manifest + each file's bytes
from the CLI (`doc_tree`/`doc_read`), holds the live **editing buffer**, and
derives outline + validity + line positions **client-side** from the buffer using
a real YAML parser (the `yaml` npm package's line-counter for exact-line jumps) —
giving instant live updates while typing (P9/P13/P18a). Within a file, outline
entries derive from the buffer instantly; **across files** (a NEW story/scenario
file created via P22, or a chat that creates one) the provider **re-fetches
`doc_tree`** so the sidebar Stories/Features list updates without a page reload
(r2 #3 — a new file cannot be produced by typing inside an existing single-file
editor). Autosave is a **debounced**
`doc_write`; the client sends it only when the buffer parses, and the CLI
re-validates as the authoritative gate. On a parse error the client keeps the
**last-valid** derived model and shows the invalid indicator (P10b). Every buffer
carries the `base_revision` it was loaded/last-saved at; each autosave sends it,
so a `doc_write` that lands on a newer on-disk revision returns a neutral
`conflict` the UI surfaces rather than silently clobbering (r1 #6/#7). Cross-source
updates (a chat edit, or `doc_read` after navigation/reload) reconcile the buffer
from disk only when the field is not being actively typed. The **conflict
resolution policy** (overwrite / merge / prompt / queue) is the brief's open
question and is DELIBERATELY left undecided — the plan builds only the detection
mechanism (revision mismatch → conflict event), not the policy.

**Design language.** Consumes the `harness/design/system/` tokens mirror (square
corners, no shadows, Poppins) with a scoped monospace exception for file views;
app-shell capped at 100vh with the content pane scrolling (Established fact — and
its right-edge-docked-element clickability gotcha is guarded, w7.3).

---

## End-to-end test cases (LEAD)

Suite: `tests/harness/`. New **`workspace` Playwright project** (test-author adds
it to `tests/harness/playwright.config.ts`, `testMatch **/w[0-9]*.spec.ts`,
~3-min timeout, no Claude) — it shares `global-setup` (image + Go binary + Redis)
with protocol/ai. Real chat-mediated edits go to a new `a*` spec (ai project).

Each spec launches container + CLI agent + a materialized fixture via a new
`helpers/workspace-env.ts` (thin wrapper over `ProtocolEnv`): start container,
`startRemote(fixtureFolder)`, poll `GET /api/sessions` until the session appears,
expose `{ sid, baseURL, fixtureDir }`, and a `readDocOnDisk(relPath)` /
`waitForDocOnDisk(relPath, predicate)` helper that reads the **fixture folder on
the host** (the CLI's cwd) — the S1 oracle. Testids + routes are added to
`helpers/ui.ts` + `helpers/README-testids.md` (test-author owns).

Fixtures (new, under `tests/harness/fixtures/`, mirroring real shapes per the
Established facts): `w-workspace-happy/` — `docs/architecture/architecture.yaml`
(mcp-service custom + postgres image + terms + docker compose_file, endpoints on
mcp-service, connection info on postgres), `docs/prd/prd.yaml`,
`docs/prd/features.yaml` (summaries, error-handling, inventory-view — id+description
only), three flat `docs/prd/stories/60.x-*.yaml` (each with `feature:`),
`docs/scenarios.yaml` (E2E-601/602/603 with `feature:`, INT-901 with **no**
feature, plus one scenario with a **dangling** `feature: ghost` for P25).

**Anti-placeholder discipline (r1 #8/#14, refined r2 #8).** (a) Cases that assert
rendered/echoed content mutate their fixture to a **unique per-run value** (random
token) so a hardcoded string cannot pass (applies wherever content is echoed:
w2.1, w3.x edits, w4.x, w5.4/w5.7 reassignment targets, chat edits). (b) The
S1/YAML oracle **parses** the committed file (not substring-matches) and asserts
exact node keys/values scoped to the right node — e.g. INT-901's `feature:` is
read from the parsed `INT-901` node, never a `/INT-901:[\s\S]*feature:/` regex a
later scenario could satisfy. (c) For "no write" cases the **authoritative**
negative oracle is a **full byte snapshot of every allowed document** before/after
(catches a write via ANY route, incl. chat); intercepting/counting the browser
doc-write route is a **diagnostic supplement**, not conclusive on its own. (d)
Derived-view assertions require the content to be **associated with the correct
service/row** (a descendant of that row's details region), absent from the wrong
one — not merely present somewhere on the page. Assertions are phrased to FAIL
when the behavior is absent (missing route → hard error; missing derived
row/persist → assertion failure).

### `w1-shell-navigation.spec.ts` — rail, flyouts, sidebar (P1–P6, S2)

- **w1.1 rail docks a section + marks it active (P1).** Click `rail-item-architecture`
  → docked sidebar shows `sidebar-section-architecture` and NOT
  `sidebar-section-product`; `rail-item-architecture` carries the active marker
  (`aria-current="page"` or `is-active`); URL is the architecture route. **S2** is
  satisfied by construction (the served app IS Next.js App Router — the container
  is built via `next build` from `harness/app/`); a light structural check asserts
  the workspace routes are served by App Router route modules (r1 #4).
- **w1.2 hover flyout previews a non-active tree; click inside navigates + docks (P2).**
  On the architecture page, hover `rail-item-product` → `rail-flyout` becomes
  visible and contains a Stories entry; click a story entry inside the flyout →
  URL is that story page, `rail-flyout` hidden, product now docked + active.
- **w1.1b all four rail sections dock, incl. Home and Builds (P1/P3 — r2 #4/#21).**
  Click `rail-item-home` → docks + active + URL is the workspace-overview route +
  the Home landing renders its overview content. Click `rail-item-builds` → docks +
  active + URL is the builds route + the Builds landing renders. The Builds landing
  is **navigation-only**: it contains no file editor and no chat mutation target
  (the gutted harness has no runs pages; none are rebuilt here).
- **w1.3 requirements under Product, Builds separate (P3).** Product docked
  sidebar contains a `Scenarios` group with `scenario-row`s; `rail-item-builds`
  exists as its own section; no `scenario-row` appears under the Builds tree.
- **w1.4 split click targets on a group row (P4).** Hover a sidebar group row →
  `sidebar-caret` appears (absent at rest); click the caret → group collapses
  (its child rows hidden) with URL unchanged; click the row **name** → navigates
  to that row's page. Two independent targets on one row.
- **w1.5 expand/collapse survives navigation (P5).** Collapse the Stories group,
  navigate to the architecture page and back → Stories is still collapsed.
- **w1.6 open page's row persistently highlighted (P6).** Navigate to story 60.2
  → its `sidebar-row` has the selected marker; navigate to scenarios → the 60.2
  row loses it and the scenarios row gains it.

### `w2-file-view.spec.ts` — GitHub-style file page + edit-in-place (P8, P17)

- **w2.1 architecture is ONE file page, GitHub-style (P8).** `file-view-path`
  text == `docs/architecture/architecture.yaml`; the gutter (`file-view-gutter`)
  has a line-number entry per content line (count == buffer line count); the
  editor contains the verbatim signature lines `services:`/`mcp-service:` AND the
  fixture's **unique per-run token** (so a hardcoded gutter+path+text placeholder
  cannot pass — r1 #8). No per-entity form pages are linked.
- **w2.2 edit-in-place, no chrome change beyond caret (P17).** Capture the
  editor's computed `backgroundColor`, `borderStyle`, `outlineStyle`,
  `boxShadow` unfocused; focus it; re-capture. Background is `rgba(0, 0, 0, 0)`
  (transparent) both states; border/outline/box-shadow are unchanged (ClickUp
  edit-in-place fact). No text reflow (the `services:` line's bounding box y is
  unchanged on focus).

### `w3-outline-jumps.spec.ts` — live outline, provenance, exact-line jumps (P13–P16)

- **w3.1 mandatory architecture outline, per-service tech stack + provenance
  (P13/P15 — r2 #20).** The **sidebar outline** shows a `Services` group with
  **exactly one entry per service** (`mcp-service`, `postgres`) and NO sub-tree
  under a service — the service outline sub-hierarchy is an open question, so the
  test asserts one-entry-per-service and does NOT prescribe nested sidebar
  children. Also a flat `Terms` group and a `Docker` group whose entry is the
  `compose_file` path. Per-service **tech stack** (mcp-service: `go`, `net/http`)
  and **Docker provenance** (mcp-service custom → its Dockerfile path; postgres
  image → its `compose_ref`) are asserted as **associated with the correct service
  in the file page's derived details region** (descendant of that service's
  details area, absent from the other service's), NOT as nested sidebar entries;
  the compose file path is listed under Docker separately (P15).
- **w3.2 outline updates LIVE as content changes (P13/P9).** Type a `redis:`
  service block into the editor; without reload the Services outline gains a
  `redis` row (client-side derivation; `arch-service-row` count increases by one).
  Then delete it → the row disappears (live removal, not just addition).
- **w3.3 outline entry jumps to the exact line, incl. cross-page (P14).** Click a
  **mid-file** service/term outline entry (chosen so the target is NOT near the
  file end, avoiding the Established clamp-at-extremes edge) → the editor scrolls so
  that line's gutter number aligns to the top within tolerance / the line-flash
  lands on that line's index. A separate assertion clicks a near-end entry and
  asserts the **clamped** target (scrollTop == max) rather than top-alignment.
  From a story page, click an architecture outline entry → navigates to
  architecture AND scrolls to the line (navigate-then-scroll; `scroll={false}`).
- **w3.4 endpoints on custom, connection info on supporting; both editable
  (P16).** The `mcp-service` row's derived view surfaces its endpoints
  (method `POST`, path `/mcp`, its summary — **descendants of the mcp-service
  row**) and NOT connection info; the `postgres` row surfaces connection info
  (`port: 5432` — descendant of the postgres row) and NOT endpoints. Then **edit**
  an endpoint into the file for mcp-service (unique path) → it appears in the
  derived view AND persists on disk (`waitForDocOnDisk`), proving endpoints are
  declared/edited, not just displayed.

### `w4-persistence.spec.ts` — CLI persistence, autosave, invalid YAML (S1, P10a, P10b) — CORE

> Order-independence (r1 #9): each case seeds its OWN unique edit in its OWN env,
> or w4.1→w4.3 are written as ONE atomic test sharing a single env (no
> cross-test fixture dependence; `workers:1`/`retries:0` do not create shared
> state). The UI exposes an **observable save state** (`data-save-state` /
> last-saved `revision`) so negative and success waits are deterministic, never a
> fixed sleep.

- **w4.1 direct edit autosaves through the CLI, proven by the two-part S1 oracle
  (S1/P10a — r1 #1).** Append a **unique** term in the editor; wait for the
  save-state to reach `saved`; then assert BOTH: (1)
  `waitForDocOnDisk('docs/architecture/architecture.yaml', parse ⇒ term present)`
  (relay ruled out — no container mount), AND (2) the browser `doc_write`
  **response** receipt AND the **test-only relay receipt-audit hook** (read by the
  workspace-env helper, keyed by `work_id`; `/api/agent/reply` itself is CLI→relay
  and NOT browser-observable — r3 #1) both carry `{path, committed_revision,
  content_hash}` matching the on-disk `SHA-256` (proves the **CLI**, not merely a
  host process, wrote it). Together: browser → relay → CLI → disk.
- **w4.2 persistence survives reload (P10a).** After w4.1's on-disk confirmation,
  `page.reload()` → the editor shows the edited content (fresh `doc_read`).
- **w4.3 persistence survives harness restart (P10a).** `env.server.restart()`
  (recreates the container); **wait for the agent to re-register** (session
  reappears via the existing session-appearance helper) BEFORE reload; then reload
  → edited content still present (the file on disk is the source of truth).
- **w4.4 invalid YAML is not committed; derived views hold last-valid (P10a/P10b).**
  Type content that breaks YAML (unterminated flow `services: [`); wait for the
  save-state to reach the observable **`invalid` / `rejected`** state (not a bare
  sleep — r1 #10); assert `readDocOnDisk(...)` byte-equals the last valid content
  AND no new `doc_write` was accepted; `yaml-invalid-indicator` visible; the
  Services outline still shows the last-valid rows. Then correct the YAML → the
  on-disk file updates, indicator clears, outline re-derives.
- **w4.5 persistence REQUIRES the CLI — two negative proofs (r1 #11 / r2 #11).**
  (a) With no connected agent (stop the remote, wait for the session to disappear
  via the existing `SESSION_GONE` helper), a UI edit's `doc_write` returns exactly
  **404 `session_gone`** (or **504 `cli_timeout`** if the request outlives the
  mutation deadline) — the named status from the api-client mapping — within the
  bounded deadline, the full-tree byte snapshot is unchanged, and the UI never
  shows a `saved` state. (b) A distinct case where the agent **dies after
  accepting** a write: the browser sees the lost-reply failure (504/`session_gone`)
  and MUST NOT claim success; because an already-delivered mutation may complete
  CLI-side (per the relay contract), the on-disk state must be consistent with the
  receipt if one was produced — the test asserts no *false* success, not that the
  write was necessarily discarded.

### `w5-product-features.spec.ts` — product docs, features, alignment (P18–P25)

- **w5.1 all four product docs are file pages (P18).** The PRD page, features
  page, story-60.2 page and scenarios page each render in the same file view:
  `file-view-path` == `docs/prd/prd.yaml` / `docs/prd/features.yaml` / the story
  file / `docs/scenarios.yaml`; each shows its verbatim content incl. its unique
  per-run token. (r1 #4: PRD + features pages were previously untested.)
- **w5.1a fixed Product outline + navigation & jumps (P18a — r1 #3).** The Product
  docked sidebar has exactly the four groups in order — `PRD`, `Features`,
  `Stories`, `Scenarios`. Row counts/labels are fixture-derived (three story rows;
  three feature rows; four scenario rows). The `Features` **header** navigates to
  the `features.yaml` file page; a **feature row** navigates to that feature's
  aggregation page; a **story row** cross-page-jumps to its file's top line; a
  **scenario row** jumps to that scenario's exact line on the scenarios page. Live
  outline (r2 #3): creating a story via the **P22 create-story UI** makes a new
  Stories row appear without reload (the provider re-fetches `doc_tree`); and
  typing a new scenario id in the scenarios editor adds a Scenarios row live (a
  within-file derivation, which typing CAN drive — unlike a new file).
- **w5.2 no epics; stories are flat (P19).** The Product Stories group lists
  `sidebar-row`s as flat siblings (all share one parent element); no
  `epic-section`/epic testid exists anywhere in the workspace.
- **w5.3 features.yaml is EXACTLY id+description; refs live on the other side
  (P20 — r1 #14).** The test **parses** `docs/prd/features.yaml` and asserts every
  record's sorted key set equals `["description","id"]` (not a presentation check);
  the parsed story file carries a `feature:` key; the parsed scenario node carries
  `feature:`. (Free-text editing stays only YAML-gated, not schema-policed, to
  respect P9 — see Challenges for that boundary note.)
- **w5.4 feature aggregation is derived live, with its description (P21 — r1 #4).**
  The `summaries` feature page shows the feature's **description** (from
  features.yaml) plus the stories and scenarios whose `feature:` == `summaries`
  (60.1/60.2 + E2E-601, per fixture) under `feature-stories-list` /
  `feature-scenarios-list`; reassign story 60.1's feature to `error-handling` two
  ways — via its **picker**, AND by **hand-editing** the `feature:` line in the
  story file view — each re-buckets it live without reload (P21 "by hand, picker,
  or chat"; the chat path is w6.3). (r2 #4)
- **w5.4a any product doc is directly editable + live + persisted (P9).** On the
  PRD file page (and the features file page), a direct edit of a unique token in
  the editor updates the view and persists on disk (`waitForDocOnDisk`, parsed) —
  proving "any workspace document" is editable/derived/persisted, not architecture
  and stories alone (r2 #4).
- **w5.5 create story requires a feature; existing-pick AND create-new (P22 —
  r1 #4).** The new-story form's feature picker is **required** (submitting without
  one is blocked). Two paths: (a) **search + pick an existing** feature — type a
  query that excludes some seeded features, assert the option list **filters**
  (nonmatching features absent, so a static non-searchable `<select>` fails — r2
  #19), then pick the survivor → the new story file is created carrying that
  `feature:` (parsed on disk); (b) **"+ Create"**
  a novel name → appends an `id`+`description`-only stub to `features.yaml`
  (parsed: the new record's keys == `["description","id"]`) AND creates the story
  file with `feature: <slug>` (exclusive-create on disk).
- **w5.6 change a feature via the picker → written to YAML (P23).** On the story
  page, change the feature via the searchable picker; assert the **parsed** story
  file's `story.feature` == `error-handling` (through the CLI).
- **w5.7 unaligned bucket + retro-tag (P24 — r1 #8).** INT-901 (no `feature:`)
  appears in the `unaligned` bucket; pick a feature for it there → assert the
  **parsed `INT-901` node** now has `feature:` set (scoped to that node, not a
  spanning regex) and INT-901 leaves the unaligned bucket.
- **w5.8 dangling feature ref surfaces as unaligned (P25).** The fixture scenario
  tagged `feature: ghost` (no `ghost` in features.yaml) shows in the
  unaligned/dangling bucket with a visible marker — not silently dropped.

### `w6-chat.spec.ts` — docked chat panel (P10, P11, P12)

- **w6.1 chat docks in-layout, narrows content, collapses to an edge tab (P11).**
  Measure the content pane width; open the chat (`chat-dock-toggle`) → the content
  pane is narrower and the `chat-dock-panel` is a flow sibling that does not
  overlap the content (content right edge ≤ panel left edge; panel is not
  `position: fixed`/overlay); collapse → the panel is gone and a thin
  `chat-dock-toggle` edge tab remains.
- **w6.2 one workspace-wide conversation follows navigation (P10).** Send a
  message on the architecture page; navigate to a story page → the same message is
  still in the history (persistent layout).
- **w6.3 chat edits the current file / converses only on non-file pages (P10 —
  r1 #8).** Using the **deterministic chat driver** (see Challenges), issue an edit
  command on the architecture page → the editor buffer updates immediately, the
  live outline re-derives, AND `waitForDocOnDisk(...)` (parsed) confirms the CLI
  persisted it, **verified by the same two-part S1 receipt oracle as w4.1** (so
  chat's S1 is proven, not just its UI effect — r2 #1). On BOTH a feature
  aggregation page AND the **Home** page, a chat
  message returns reply text while a **snapshot of every allowed document** is
  byte-unchanged and the intercepted doc-write route received **zero** `doc_write`
  (no single backing file → no edit).
- **w6.4 resize the chat by dragging its divider (P12).** Using bounding-box
  pointer events (not an approximate delta — r1 #10), drag `chat-dock-resizer` by
  a measured Δ → the `chat-dock-panel` width changes by ≈Δ.
- **w6.5 live derivation on a PRODUCT file, by hand and by chat (P9).** On the
  scenarios file page, typing a new scenario id updates the Scenarios outline live;
  a deterministic chat edit to the same file also updates it live and persists —
  proving "any workspace document" derives live, not architecture alone (r1 #4).

### `w7-shell-quality.spec.ts` — design system + app-shell scroll (P7, Established facts)

- **w7.1 consumes the S&F token mirror (P7).** A documented token from
  `harness/design/system/tokens.css` is applied to a real workspace element AND
  tracks a live mutation of the token at `:root` (mirrors m1.6c — defeats a
  hardcoded literal).
- **w7.1a ClickUp parity matrix (P7 — r1 #5).** Token plumbing ≠ interaction
  parity. A parity table maps each **measured** behavior in `clickup-reference.md`
  to a gating assertion; most ride existing cases (caret swap/split-click → w1.4;
  flyout placement + close-on-mouseout → w1.2; chat in-layout reflow/no-shadow →
  w6.1; edit-in-place computed styles → w2.2). This case adds the still-uncovered
  testable ones: sidebar rows have **no caret at rest** and the icon slot swaps to
  a caret **on hover**; caret glyph direction (`▸` collapsed / `▾` expanded);
  child indentation guide line present; rail utility items **pinned at the rail
  bottom**; the **flyout floats immediately right of the rail, over content**;
  the chat panel has its **own header row (new-chat control)** and its input
  **pinned at the panel bottom**; chat panel default width is the measured wide
  (~40% at 1920). The table explicitly **classifies** the remaining measured
  behaviors — trailing hover quick-actions and the floating edit toolbar (the
  brief calls it optional) — as *intentionally adapted/deferred*, so no entry is
  left silently unmapped (r2 #5); purely visual parity stays a design-review item.
- **w7.2 monospace file-view exception.** The editor/gutter computed `font-family`
  is a monospace stack while `document.body` resolves to Poppins.
- **w7.3 app-shell scroll + right-edge clickability (Established fact).** With a
  long file scrolled, the body does not scroll (`documentElement.scrollHeight ≈
  clientHeight`; the content pane owns the scroll) and the right-edge
  `chat-dock-toggle` is still clickable (click toggles it) — guarding the
  documented "docked element hides under the body scrollbar" gotcha.

### `a10-workspace-chat-edit.spec.ts` — real chat-mediated edit (ai project, P10, S1)

- A real Claude chat turn on the architecture page that requests a concrete YAML
  change → the change appears in the editor AND
  `waitForDocOnDisk(...)` confirms CLI persistence. Proves the production
  chat→doc_write→CLI path end-to-end (the deterministic w6.3 covers the path
  cheaply; this proves the real integration). Counts toward the AI-call budget.

---

## Startup scaffolding (created empty / behavior-free by the test-author)

Per paths.md the test-author may create files that let services START but leaves
them behavior-free for the coder. The harness container must serve a 200 on `/`
for readiness and its routes must resolve, so:

- `harness/app/layout.tsx` — root HTML layout shell. No workspace logic.
- `harness/app/page.tsx` — sessions-list placeholder that returns 200 (container
  readiness only; the real list is coder work).
- `harness/app/api/agent/{register,poll,reply}/route.ts` — route files returning
  501 Not Implemented. Present so the agent-protocol endpoints resolve and the
  coder fills them; no coordination logic.
- `harness/app/api/sessions/route.ts` and
  `harness/app/api/sessions/[sid]/docs/{route.ts,read/route.ts,write/route.ts}` —
  501 stubs. Present so the document routes resolve to a real (failing) response,
  not a 404, and the coder fills them.
- `harness/app/sessions/[sid]/(workspace)/layout.tsx` + a placeholder
  `home/page.tsx` — the persistent route-group skeleton (renders children only)
  so the workspace routes resolve. No rail/sidebar/file logic.
- `tests/harness/playwright.config.ts` — add the `workspace` project entry (test
  config, not behavior).
- `tests/harness/helpers/workspace-env.ts`, additions to `helpers/ui.ts` and
  `helpers/README-testids.md`, and the `tests/harness/fixtures/w-*` trees — test
  assets (test-author territory), not production behavior.

Each contains no production logic: the red specs fail on missing behavior/persist
(assertions), not on unresolved routes or a dead container.

## Implementation (production changes by service/layer)

All specs are authored RED first (tests-first invariant); the coder (Sonnet) then
greens them in **vertical slices** so each step turns a specific w-cluster green
rather than one long all-red interval (r1 #13). Unit tests (Go / Vitest) precede
each slice's e2e. Guard: build ONLY the doc/chat surface — never the run/prompt/
build machinery.

- **Slice 0 — persistence backbone (green: `doc_read` smoke).**
  - *CLI (Go — `src/internal/app/remote/`):* add `doc_tree` / `doc_read` /
    `doc_write` synchronous work items + handlers (parallel to the `query` handler
    in `handlers.go`, off the run machinery). `doc_write` implements the full
    contract from Target state: **path-safety** (workspace-relative, canonical
    containment under `docs/`, symlink resolution, `.yaml`-only, glob allowlist
    with exclusive-create for new `docs/prd/stories/*.yaml`), **YAML gate**,
    **per-document serialization + stale-`base_revision` conflict**, **atomic
    temp+fsync+rename**, and a **write receipt** `{path, committed_revision,
    content_hash, bytes}` (r1 #1/#6/#12; r2 #6/#12/#16/#17/#18). Explicitly
    **classify the new work types** — add `ClassDoc` and a separate chat lane in
    `pool.go`/`scheduler.go` (r3 #6; the default `ClassRead` mapping would run
    writes as bounded reads, and reusing `ClassDispatch` would let a slow chat
    stall runs). Direct file I/O ("no loaders/caching"). Go tests:
    traversal/symlink-escape (final target too)/absolute/NUL/wrong-dir reject,
    duplicate story path, permitted new story, YAML-gate, stale-revision after
    restart AND after external edit, reversed/delayed replies, duplicate token vs
    differing-args-409, read-during-write, dir-fsync/perms/temp-cleanup.
  - *Relay (Next — `harness/app/api/`):* `register/poll/reply` + Redis
    coordination (epoch, capability token) only to the extent the workspace needs
    (one connected session via `GET /api/sessions`), reusing the documented
    contract in `helpers/api-client.ts`/`README-testids.md`. Session-scoped `docs`
    tree/read/write routes enqueue the document work items and propagate the
    receipt to the browser; the relay never touches the filesystem; `doc_write`
    enforces exact Origin+Host before business logic. Add the **test-only
    receipt-audit seam** (r3 #1): behind a test flag/env, the relay records each
    CLI receipt keyed by `work_id` in Redis (bounded TTL) exposed via a read-only
    audit route the workspace-env helper reads for the S1 oracle — gated off in
    production. The test-author adds the reader to `helpers/workspace-env.ts`.
- **Slice 1 — architecture page + FileView (green: w2, w3).** Persistent
  `(workspace)` layout + `FilesProvider` (load via `doc_read`, editing buffer,
  client-side `yaml`-parser derivation with line positions, debounced autosave via
  `doc_write` with `base_revision`, last-valid retention + invalid indicator,
  observable `data-save-state`, conflict surfacing, reconcile-when-not-typing).
  `FileView` (path header, line gutter, edit-in-place no-chrome focus + monospace
  exception, invisible line anchors + flash, clamp at extremes). Architecture
  outline (Services one-per-service with tech stack + custom/image provenance,
  flat Terms, Docker compose path; endpoints/connection-info as descendants of the
  right service; exact-line jumps, `scroll={false}`).
- **Slice 2 — persistence + shell (green: w4, w1, w7).** Wire autosave/invalid/
  restart end-to-end; the icon `Rail` (dock + active), hover `Flyout`, docked
  `Sidebar` split-click rows (caret toggle / name navigate) with expand state in
  the persistent layout (P5); design-system tokens; app-shell capped at 100vh with
  the content pane scrolling and right-edge elements clickable.
- **Slice 3 — product docs + outline (green: w5.1–w5.3, w5.1a).** PRD, per-story,
  scenarios file pages on the same `FileView`; product outline (PRD, Features
  header → features.yaml page, flat Stories with per-file jumps, Scenarios with
  per-line jumps). No epic level.
- **Slice 4 — features & alignment (green: w5.4–w5.8).** Derived `/features/<id>`
  aggregation pages; shared searchable `FeaturePicker` (existing pick OR inline
  create → id+description stub appended to features.yaml via `doc_write`);
  new-story form (feature required, exclusive-create); unaligned bucket (no-feature
  + dangling-ref) with in-place retro-tagging → `doc_write`.
- **Slice 5 — chat (green: w6, a10).** Docked, in-layout, content-narrowing,
  resizable, edge-tab-collapsible chat in the persistent layout (one conversation
  following navigation). New **chat work item** with the DTOs from Target state →
  Chat transport (`{conversation, current_path, current_content}` →
  `{reply_text, edit:{path,new_content}|null}`, full-content result, target
  binding, malformed/YAML-invalid/timeout branches, write scheduler class): CLI
  runs one Claude turn (`src/claudecode/`); browser applies `edit.new_content` to
  the buffer, autosave `doc_write` persists it (same S1 path); non-file pages
  converse only. A **deterministic chat driver** (hidden, mirroring `prompt-probe`)
  returns a scripted structured result so protocol tests exercise the
  buffer→doc_write path with no model call; real Claude for `a10`. Go tests for the
  malformed-result / wrong-target / YAML-invalid / timeout-cancellation branches
  precede `a10`.
- **Package manifest.** Coder adds the `yaml` runtime dependency to
  `harness/package.json` **dependencies** (allowed); never touches `scripts`.

## Codex rounds

See the ledger beside this plan: `docs/tasks/plans/workspace-file-as-source-ui.codex.md`.

## Challenges

- **The S1 architecture is a genuine cross-service decision.** Recommended:
  extend the relay agent protocol with document work items. Ratify in review/Codex.
  The S1 proof is **two-part** (r1 #1 / r3 #1): the unmounted container rules out
  the relay, and the **CLI write receipt** (`committed_revision` + `content_hash`)
  — read from the browser `doc_write` response AND the test-only relay
  receipt-audit hook keyed by `work_id` (`/api/agent/reply` itself is CLI→relay,
  not browser-observable) — proves the CLI, not merely some host process, wrote
  the file. The unmounted-container fact
  alone is necessary but not sufficient. Alternative (harness bind-mounts + shells
  `true-bdd docs`): simpler but even the receipt can't exclude a direct Next write,
  and the deployed model breaks.
- **Persistence robustness (r1 #6/#12).** A CLI filesystem write surface needs
  path-safety (traversal/symlink/absolute/extension), per-document serialization,
  stale-`base_revision` conflict detection, idempotent token replay, and atomic
  temp+fsync+rename — all specified in Target state / Slice 0. Underspecifying any
  of these is a real correctness/security gap, not polish.
- **Greenfield relay.** The harness app and its register/poll/reply core do not
  exist. Scope discipline is essential: build only agent core + session listing +
  document/chat work items, NOT the run/prompt/build machinery. Risk of over-build.
- **Chat transport + deterministic driver (r1 #2).** The production chat path is
  specified (chat work item → Claude turn → structured result → buffer →
  `doc_write`), so `a10` is implementable. Protocol tests use a hidden scripted
  driver (like `prompt-probe`) that returns a canned structured result, exercising
  the identical buffer→doc_write path with no model call; it must not leak into
  production UX (gate it). Confirm with the reviewer.
- **Open questions — planned around, not resolved (r1 #7).** Chat-vs-typing
  autosave conflict → the plan builds ONLY the detection mechanism
  (`base_revision` mismatch → neutral `conflict` event); the resolution policy
  (overwrite / merge / prompt / queue) is DELIBERATELY undecided. Feature rename
  propagation → P25's dangling-as-unaligned path is built; auto-rewrite of refs is
  NOT. features.yaml schema boundary → the inline-create STUB is asserted to be
  exactly id+description, but free-text editing stays only YAML-gated (never
  schema-policed, respecting P9); whether persistence should enforce the feature
  schema is left to review. prd `stories:` auto-sync, compose generation, service
  outline sub-hierarchy, error/empty states beyond invalid YAML → out of scope.
- **Flake risks (r1 #9/#10/#11).** (1) Hover flyout (P2) — respect the ~150ms
  open/close delay, move the pointer rail→flyout, `waitFor` visible + assert
  delayed close after mouseleave. (2) Autosave/negative — wait on the observable
  `data-save-state` / `revision` and `waitForDocOnDisk` (parsed), never a fixed
  sleep; test-configurable debounce clock. (3) Line jumps — pick a **mid-file**
  target to avoid the Established clamp-at-extremes edge; assert the clamped target
  for near-end entries. (4) Resize — bounding-box pointer events, measured Δ.
  (5) Restart — wait for agent **re-registration** before reload. (6) Negative S1 —
  use the existing `SESSION_GONE` helper + exact status code + bounded deadline +
  before/after snapshot. (7) app-shell right-edge clickability (w7.3) is exactly
  the documented "invisible under body scrollbar" trap — keep a hard click assert.
- **Order-independence (r1 #9).** w4.1–w4.3 are one atomic test (or per-case
  independent seeds); Playwright `workers:1`/`retries:0` do NOT create shared
  fixtures across specs.
- **Derivation correctness.** Use the `yaml` parser's line-counter for exact-line
  outline entries and for the invalid-YAML gate; do not reuse the prototype's
  line-heuristic regex parsing (brittle on real files).
- **`next/link scroll={false}`** wherever a custom post-navigation scroll runs
  (Established fact: default scroll reset races the jump).
- **CSS gotchas (trust the appendix).** `overflow-x: hidden` forces computed
  `overflow-y: auto`; `input:not([type])` has (0,1,1) specificity beating a plain
  class — bear both in mind for the file-view/editor styles.

- **Challenge — w3.3 test-side import bug (Phase 2 blocker).** Failing assertion:
  none reached — `TypeError: (0, _workspaceEnv.lineIndex) is not a function`
  (spec imports `lineIndex` from `./helpers/workspace-env`; it is exported from
  `./helpers/ui` — ui.ts:460). Constraint: coder is hook-blocked from `tests/`;
  Codex read-only consult (`./tmp/codex-w3-lineindex.md`) verdict STOP —
  unfixable from production code. Orchestrator verified the import mismatch
  directly. Recommendation: one-line test-side import fix by the test-author.
  User decision (AskUserQuestion): **approved the test fix**. Next action:
  test-author corrects the import; orchestrator re-runs the suite isolated.
- **Challenge — two real production bugs found by the coder during a10
  verification** (fixed in production, no test changes): (1) chat route used the
  30s `DEADLINE_MS.inventory` for real Claude turns → spurious 504 before the
  reply; added dedicated `DEADLINE_MS.chat` (20 min). (2) Claude wrapped
  `edit.new_content` in YAML `---` document markers; Go lenient-parsed it but the
  browser's strict `YAML.parse` rejected it, silently discarding correct edits;
  added `stripAccidentalDocumentMarkers` in `src/internal/app/remote/chat.go`
  with Go regression tests reproducing the captured payload.

- **Retro inputs (user-requested research, 2026-08-04 — process at close):**
  (1) *w3.3 root cause*: a test-INTERNAL wiring bug (spec imported `lineIndex`
  from `workspace-env`; it lives in `ui`), NOT an implementation dependency.
  It was invisible during the red phase because every spec failed at the
  `env.start()` session gate BEFORE the broken call site, and the red sample ran
  w3.1, not w3.3. Systemic gap: `tests/harness` has NO tsconfig/typecheck —
  Playwright transpiles without type-checking, so a missing named export
  compiles and only explodes at call time. Proposal: add a `tsc --noEmit` static
  gate to the test package + the test-author's definition-of-red.
  (2) *Escalation policy*: the skill routes ANY test change to the user via
  AskUserQuestion. Right guard for behavioral changes (assertion weakening —
  the core threat), noise for mechanical repairs (imports/typos/syntax) already
  Codex-confirmed as non-behavioral. Proposal: orchestrator may approve
  NON-BEHAVIORAL test repairs (zero assertion/expected-value changes) with
  evidence logged in Challenges; user escalation reserved for semantic changes.
  (3) *Stop-gate false block*: phase_state's stop gate measured from coder
  completion and repeatedly blocked turn-end while a freshly-spawned reviewer
  was legitimately running in the background (~1 min old), eventually recording
  a spurious auto_block. Proposal: the gate should treat a recorded reviewer
  spawn (with a live transcript) as satisfying the window, or size the window
  to review reality (tens of minutes on the hard lane).

## Workflow log

- Planner: read paths.md, brief, requirements/terms, the prototype
  (FilesStore/FileView/sections/ChatDialog/ProductFiles/FeaturePicker/feature
  page), clickup-reference, the e2e infra (ServerController/RemoteProcess/
  ProtocolEnv/global-setup/api-client/ui/README-testids), CLI remote (remote.go/
  wire.go), and compose files. Established: harness app is greenfield (ba52d00);
  relay model + unmounted container gives the S1 proof. Wrote this plan.
- Codex critique loop: 3 rounds run (hard-lane cap), 44 findings kept / 0 skipped
  — R1 tightened S1 proof + assertion strength + open-question fiat; R2 caught
  feasibility gaps (scheduler `ClassRead` default, `/api/agent/reply` not
  browser-observable, revision-as-counter) + coverage holes (P18a, Home/Builds);
  R3 finalized the receipt-audit seam, scheduler `ClassDoc`/chat lane, chat
  timeout test, and durable-write ordering. See the ledger for the round tables.
- Orchestrator (2026-08-03): plan reviewed — architecture ratified for Phase 1.2
  (S1 via relay document work items; two-part receipt oracle; slices 0–5).
  Test-author spawned (hard lane, ≤3 Codex rounds).
- Orchestrator (2026-08-04): test-author done — 36 workspace specs + a10, 3 Codex
  rounds (17 applied). Scope check PASSED: production diff = exactly the 11
  plan-listed scaffolding files; scripts untouched. Red confirmed (7/7 sampled
  specs fail at the 501 session gate; logs ./tmp/workspace-red*.log). Off-limits
  + scripts before-coder snapshots taken. Coder spawned (Phase 2).
- Orchestrator (2026-08-04): coder done — slices 0–5 implemented; isolated rerun
  35/36 (only w3.3 red = test import bug) + a10 1/1 on real Claude; two real
  production bugs found+fixed during a10 verification (chat deadline, YAML
  document-marker stripping — see Challenges). Off-limits sources + scripts
  verified CLEAN post-coder. Orchestrator's concurrent suite run was
  resource-contaminated (3 flakes) — final authoritative run scheduled after the
  w3.3 fix. User APPROVED the w3.3 test-side import fix; test-author re-spawned
  for that one line.
