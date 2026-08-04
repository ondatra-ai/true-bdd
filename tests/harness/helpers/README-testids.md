# Harness v2 UI + API contract (defined by the protocol specs)

The protocol Playwright specs (`p1-*.spec.ts` … `p16-*.spec.ts`) are
authored BEFORE any feature code (tests-first). This file is the contract
the CLI store, the stateless relay, and the UI phases implement to. The
typed source of truth is `helpers/api-client.ts` (routes, bodies, status
codes, agent protocol) and `helpers/ui.ts` (routes, testids); this document
is the human summary.

## v2 architecture (plan §1–§4, critique)

- **The CLI owns ALL state** in a per-project SQLite store at
  `<folder>/tmp/true-bdd-state.db`. The Next server is a **stateless relay**
  with **no database**.
- **Sessions are GONE on disconnect.** Every session in `GET /api/sessions`
  is connected by definition — there is no `unreachable` state, no
  reachability overlay, and no mark-abandoned control.
- **No generation, no cache.** Every browser read is a fresh CLI scan. The
  Refresh button triggers an immediate live `session_detail` read, not a
  mutation.
- **Run routes are session-scoped** (a global `/api/runs/:id` is impossible
  without server state): `/api/sessions/:sid/runs/:rid`.
- **Prompts are native `<dialog>` modals** answered via RPC.

## Navigation

| Route | View |
|---|---|
| `/` | Sessions list |
| `/sessions/<session-id>` | Session detail (inventory, epics, stories, actions, run history) |
| `/sessions/<sid>/runs/<rid>` | Run view (output tail, prompt DIALOG, outcome badge) |

All three views **live-poll** their API. `usePoll` returns
`{data, status, error}`: on `404 session_gone` the page clears data and
navigates to the sessions list (or a terminal disconnected view); on
`504 cli_timeout` it renders an explicit **unavailable** state — stale data
is never silently presented as current.

## Browser API surface (session-scoped)

| Route | Success | Errors |
|---|---|---|
| `GET /api/sessions` | 200 `{sessions: SessionSummary[]}` | 403 wrong Host |
| `GET /api/sessions/:id` | 200 `SessionDetail` (status + inventory) | 404 session_gone, 504 cli_timeout, 502 invalid_cli_reply, 413, 429/503 |
| `GET /api/sessions/:id?view=status` | 200 `SessionStatus` | 404/504/502/429/503 |
| `POST /api/sessions/:id/runs` `{command, story_id?, fix, client_token}` | 201 `{run_id}`; 200 `{run_id}` on client_token dedup | 400 invalid; 409 CLI domain conflict (nonterminal run per owner OR token reuse with different args); 403 origin/host; 404 session_gone; 504 cli_timeout |
| `GET /api/sessions/:sid/runs/:rid` (`?after_seq` reserved) | 200 `RunDetail` (entire capped window) | 404 session_gone; 504 cli_timeout |
| `POST /api/sessions/:sid/runs/:rid/answer` `{prompt_id, value}` | 200 accepted / exact retry | 409 conflicting/terminal-race/cross-owner; 400 invalid; 404 session_gone/run_pruned |

- **`command` enum** (exact strings): `version`, `us-create`, `us-refine`,
  `us-apply`, `build-tests`, `build-code`, and the hidden **`prompt-probe`**
  (deterministic non-Claude choice→clarify→freetext driver). Anything else → 400.
- **`SessionSummary`** (registry-only — critique §2): `session_id`
  (client-owned, process-stable incarnation id), `folder` (canonical
  **realpath**), `pid`, `version`, `connected_at`. NO reachability, NO
  active_run_id, NO inventory_generation.
- **`SessionStatus`** (one `session_status` CLI work item): `session_id`,
  `owner_id`, `active_run: RunSummary|null`, `runs: RunSummary[]` (project
  history, newest first), `active_owners: {owner_id, session_id|null,
  run_id, command}[]` (same-project active owners — drives the sibling
  warning; no relay joins).
- **`SessionDetail`** = `SessionStatus` + `inventory` (one `session_detail`
  CLI work item — status + inventory in ONE consistent read).
- **`RunSummary`/`RunDetail`**: `run_id`, `owner_id` (distinct from the
  serving session), `command`, `story_id`, `fix`, `state`, `outcome`,
  `error_detail` (`spawn|no_result|contradiction|folder_locked`),
  `answerable` (cross-owner guard — true only for the owning live session),
  `created_at`, `updated_at`. `RunDetail` adds `session_id`, `events`,
  `pending_prompt`, `envelope`.
- Run `state`: `queued → claimed → running ⇄ prompt_published →
  answer_accepted → answer_consumed → terminal`. `outcome` (== the badge
  classification): `ok`, `converged`, `not_fixed`, `user_exit`,
  `max_attempts`, `interrupted`, `abandoned` (from boot reconciliation),
  `error`.

## Agent protocol (register / poll / reply, plan §2)

| Route | Body / headers | Result |
|---|---|---|
| `POST /api/agent/register` | `{session_id, folder, canonical_folder, pid, start_identity, version}` | `{connection_epoch, reply_budget_bytes, capability_token}` |
| `POST /api/agent/poll` | `{session_id, connection_epoch, capability_token}` (held ≤5s) | 204 nothing; 200 work item `{work_id, session_id, connection_epoch, type: query\|dispatch\|answer, payload, deadline}` |
| `POST /api/agent/reply` | correlation in HEADERS (`x-session-id, x-connection-epoch, x-capability-token, x-work-id`); body = result | 200; 413 `reply_too_large` (streamed cap, correlated); stale-epoch rejected |

- Reconnect: on poll 404 / stale epoch ⇒ re-register with the SAME
  client-owned `session_id`; a new register bumps the epoch and invalidates
  the old one; capped full-jitter backoff.
- Operation deadlines: 5s status/run-detail, 10s mutations, 30s inventory.
- Relay lifecycle: `queued → delivered → replied | cancelled | expired |
  client_aborted`. On expiry (10s absence-of-poll; an open poll counts as
  liveness) the relay atomically removes the session, cancels its poll,
  fails all waiters, rejects stale-epoch replies, and DROPS queued mutations
  (never store-and-forward). A browser abort cancels every UNDELIVERED
  operation (reads AND mutations); an already-delivered mutation completes
  CLI-side.

## Origin + Host policy (plan §2/§3.8, asserted by P6/P6b)

- Browser mutation routes (`POST …/runs`, `…/runs/:rid/answer`): require the
  exact allowed Origin AND Host; missing Origin, `Origin: null`, foreign
  Origin, or wrong Host → 403.
- Agent routes (`/api/agent/register|poll|reply`): loopback Host required;
  any PRESENT foreign Origin (incl. literal `null`) → 403; absent Origin
  accepted; non-JSON content-type → 415 before state access.
- GETs: Host check only → 403 on wrong Host.
- 403 is decided BEFORE business logic.

## Behavioral constants the tests rely on

- **Disconnect threshold**: a session with no poll for the expiry window
  (~10s; an open poll counts as liveness) is REMOVED from the registry
  (P2/P11). Reachability never existed in v2.
- **`version` run output**: the run view output contains
  `true-bdd version <something>` (`/true-bdd version \S+/`); terminates `ok`.
- **Disabled controls** are real `disabled` buttons; controls are disabled
  ONLY for the session's OWN active run — a same-project SIBLING owner shows
  the warning banner with controls ENABLED (P5); the folder flock decides at
  dispatch time (command-vs-command is fail-fast `folder_locked`).

## data-testid contract

### Sessions list (`/`)

| testid | Notes |
|---|---|
| `session-row` | One per session; `data-session-id`, `data-folder`. |
| `session-folder` | Text = canonical folder (realpath). |
| `session-version` | Text = the remote version. |
| `test-connection` | Per-row control; dispatches a `version` run (P8). |

### Session detail (`/sessions/<id>`)

| testid | Notes |
|---|---|
| `refresh` | Triggers an immediate live `session_detail` READ (a fresh scan — plan §1.5), NOT a mutation. |
| `unavailable-state` | Explicit disconnected / `504 cli_timeout` view (critique §13). |
| `folder-warning-banner` | Visible when `session_status.active_owners` has a DIFFERENT owner active (P5). |
| `path-mismatch-warning` | Configured architecture path differs from the canonical default (P4). |
| `run-history` / `run-row` | `run-row` carries `data-run-id`, `data-command`; its link is session-scoped (P7). |
| `action-build-tests`, `action-build-code` | Session-level `<button>`s; disabled only for the own active run (P5). |
| `action-create`, `action-refine`, `action-apply`, `fix-toggle` | Per-story-row action controls (AI specs). |
| `inventory-doc-<key>` | Status chip; `data-status` ∈ `present`, `missing`, `invalid`, `not_a_dir`, `present_empty`, `ambiguous`, `unknown`. Keys: `config`, `prd`, `architecture`, `registry`, `stories-dir`, `epics-dir`, `checklist-<stem>`. |
| `inventory-truncated-banner` | Visible when the inventory read was degraded to fit the negotiated reply budget (`snapshot_truncated`, any omission, or `limit_too_small`) — the fit ladder stays, retargeted to the reply envelope (critique §12). |
| `epic-section` / `epic-toggle` / `epic-title` / `epic-row` / `epic-flag-*` | Epic accordion (unchanged from v1). |
| `story-row-<create-id>` / `story-title` / `story-created` / `story-applied` / `story-refined` / `story-flag-*` | Story rows + flags (unchanged from v1). |

### Story review modal (`/sessions/<id>`, opened from `story-title`)

Unchanged from v1 — native `<dialog>` + `showModal()`. `story-modal`,
`story-modal-panel`, `story-modal-title`, `story-modal-close`,
`story-modal-tab-review`/`-raw`, `story-modal-panel-review`/`-raw`,
`story-modal-statement`, `story-modal-ac` (`data-ac-id`), `story-modal-step`
(`data-kind`), `story-modal-raw` (byte-exact), `story-modal-error`,
`story-modal-notice` (`data-reason` incl. `changed_on_disk` — surfaced by the
next live poll, no refresh route).

### Run view (`/sessions/<sid>/runs/<rid>`)

| testid | Notes |
|---|---|
| `run-state` | Text = current run state. |
| `run-outcome` | Text = outcome badge (`ok`, `converged`, …, `abandoned`, or `error(<detail>)`). |
| `run-output` | Output tail (retention gap markers rendered). |
| `run-envelope-engine-outcome`, `run-envelope-finalization`, `run-envelope-exit-code`, `run-envelope-signal` | Terminal-envelope diagnostics; present once terminal. |

### Prompt DIALOG (`/sessions/<sid>/runs/<rid>`, plan §4 — PROMPTS BECOME DIALOGS)

The inline PromptPanel is REPLACED by a native `<dialog>` opened with
`showModal()` (same pattern as the story modal). A failed answer RPC keeps
the dialog OPEN with a visible error; Escape does NOT answer (the dialog
stays until the prompt is answered or the run ends).

| testid | Notes |
|---|---|
| `prompt-dialog` | The `<dialog>`; `data-kind` ∈ `choice`, `clarify`, `freetext`; `data-prompt-id` (distinct per prompt — A2). |
| `prompt-dialog-panel` | Inner panel; a click on it must NOT answer/close. |
| `prompt-dialog-title` | Heading; `aria-labelledby` target. |
| `prompt-dialog-error` | Visible after a FAILED answer RPC; the dialog stays open. |
| `prompt-choice-apply` / `-refine` / `-exit` | Choice-prompt buttons (`kind="choice"`). |
| `prompt-answer-input` / `prompt-answer-submit` | Single-line clarify answer (the option NUMBER; the collector maps it to the option text — A3). |
| `prompt-clarify-option` | One chip per numbered clarify option; `data-index` (1-based). |
| `prompt-freetext-input` / `prompt-freetext-submit` | MULTILINE `<textarea>` refinement feedback (`kind="freetext"`, A2). |

**AI-call budget** (AI specs, NOT a UI testid): every A-test counts the
`claude` processes spawned as descendants of its remote and fails a
single-check fixture that exceeds a small per-test bound.

---

# Workspace UI + API contract (defined by the `w*` specs + a10)

The workspace file-as-source UI (`w1`–`w7`, `a10`) is authored tests-first; this
section is the binding contract the test-fixer implements to. The typed source of
truth is `helpers/ui.ts` (`WTID`, `wsRoutes`, `WorkspaceSection`, locator +
action helpers) and `helpers/workspace-env.ts` (the env + S1 oracle). It gates
requirements S1–S2 and P1–P25.

## Architecture (why these shapes)

- **S1 persistence** goes browser → relay → **CLI** → disk. The relay never
  touches the filesystem; the CLI owns every committed write. The e2e container
  has **no host-folder mount** (`docker-compose.test.yml` has no `volumes:`), so
  an on-disk change proves the relay did NOT write it.
- The **S1 oracle is two-part**: (1) `waitForDocOnDisk(...)` (parsed) — relay
  ruled out; AND (2) the **CLI write receipt** `{path, committed_revision,
  content_hash}` matching the on-disk `SHA-256`, read TWO ways — the browser
  `doc_write` **response** body AND the **relay receipt-audit hook** keyed by
  `work_id`. `/api/agent/reply` is CLI→relay and NOT browser-observable, so the
  audit hook is the only browser-side window onto the CLI receipt.

## Browser API surface (session-scoped, additive to the protocol contract)

| Route | Success | Errors |
|---|---|---|
| `GET /api/sessions/:sid/docs` | 200 `doc_tree` (fixed-schema manifest: path → node + `revision`) | 404 session_gone, 504 cli_timeout |
| `GET /api/sessions/:sid/docs/read?path=` | 200 `{content, revision, parse_status}` | 404, 504 |
| `POST /api/sessions/:sid/docs/write` `{path, content, base_revision, client_token}` | 200 `{receipt:{path, committed_revision, content_hash, work_id, bytes}}` | 404 `{error:"session_gone"}`, 504 `{error:"cli_timeout"}`, 409 conflict (stale `base_revision` / token-reuse-different-args), 422 invalid YAML, 403 origin/host |
| `GET /api/_test/receipts?sid=` | 200 `{receipts: WriteReceipt[]}` — each `{path, committed_revision, content_hash, work_id}` (test-only; enabled by `HARNESS_RECEIPT_AUDIT=1`; OFF in production) | — |

- Every receipt (browser response AND audit record) carries `work_id`; the S1
  oracle correlates the two by **exact `work_id`** (not by path/hash, which an
  unrelated identical-bytes write could satisfy), then matches `path` /
  `committed_revision` / `content_hash` against the on-disk `SHA-256`.
- The `doc_write` error body carries the exact mapped `error` string
  (`session_gone` for 404, `cli_timeout` for 504).

- `revision` is **content-derived** (`SHA-256(bytes)` + existence), recomputed
  under the per-document lock — never an in-memory counter.
- `doc_write` allowlist (exact patterns): `docs/architecture/architecture.yaml`,
  `docs/prd/prd.yaml`, `docs/prd/features.yaml`, `docs/scenarios.yaml`,
  `docs/prd/stories/*.yaml` (the last exclusive-creatable for a NEW story).
- `doc_write` is a mutation → exact **Origin+Host** enforced before business logic.

## Navigation (`wsRoutes`; the `(workspace)` route group is elided from the URL)

| Route | View |
|---|---|
| `/` | Sessions list |
| `/sessions/:sid/home` | Workspace overview (Home landing; no backing file) |
| `/sessions/:sid/architecture` | architecture.yaml file page |
| `/sessions/:sid/product` | PRD (prd.yaml) file page = Product root |
| `/sessions/:sid/product/features` | features.yaml file page |
| `/sessions/:sid/product/features/:id` | feature aggregation (derived, no file) |
| `/sessions/:sid/product/stories/:storyId` | story file page |
| `/sessions/:sid/product/scenarios` | scenarios.yaml file page |
| `/sessions/:sid/builds` | Builds landing (navigation-only; no editor/chat target) |

Workspace routes are **App Router route modules** (S2); the served HTML carries
the RSC flight marker `__next_f`. The persistent `(workspace)` layout (the shell:
`FilesProvider`, rail, sidebar, chat) survives client-side navigation.

## data-testid contract

Section keys (`WorkspaceSection`): `home` · `architecture` · `product` · `builds`.

### App shell + icon rail
| testid | Notes |
|---|---|
| `app-shell` / `content-pane` | 100vh frame; the CONTENT pane owns the scroll (body must not scroll). |
| `workspace-main` | Column wrapping the persistent breadcrumb + `content-pane`, right of the sidebar. |
| `content-breadcrumb` | Persistent breadcrumb bar above the canvas (design/SPEC.md §1 three-region frame); hairline bottom border; token-only type/colour. Trail derived by `lib/workspace/breadcrumb.ts`; last crumb `aria-current="page"`, the routeless `stories` crumb is plain text (no index route). |
| `rail` | Narrow dark far-left rail; `+ data-active-section`; consumes `--surface-inverse`. |
| `rail-item-<section>` | Per section; `+ data-section`; active carries `aria-current="page"`. |
| `rail-flyout` | Hover preview of a NON-active section's tree; floats immediately right of the rail, over content; ~150ms open/close delay. |
| `rail-utilities` / `rail-utility-item` | Utility items pinned at the rail BOTTOM. |

### Docked sidebar
| testid | Notes |
|---|---|
| `sidebar` | The docked section tree. |
| `sidebar-section-<section>` | One per section; only the docked section is present. |
| `sidebar-group` | Collapsible group; `+ data-group` = `Services`/`Terms`/`Docker`/`PRD`/`Features`/`Stories`/`Scenarios`. |
| `sidebar-group-name` | The group header's NAME link (navigates); `+ data-selected` when it is the open page. |
| `sidebar-caret` | Hover-revealed toggle (absent at rest); `+ data-expanded` (`true`/`false`); glyph `▾` expanded / `▸` collapsed. |
| `sidebar-group-body` | The group's child-row container (hidden when collapsed). |
| `sidebar-guide-line` | Thin child-indentation guide line. |
| `arch-service-row` / `arch-term-row` / `arch-docker-row` | Architecture outline rows (`+ data-service` / `+ data-term`; docker row text = compose_file path). ONE row per service, no nested sub-tree. |
| `prd-row` / `feature-row` / `story-row` / `scenario-row` | Product outline rows (`+ data-feature` / `+ data-story-id` / `+ data-scenario-id`). Every navigable row carries `+ data-selected` (`true`/`false`) for the open-page highlight (P6). No `epic-*` testid exists anywhere in the workspace (P19). |

### GitHub-style file view
| testid | Notes |
|---|---|
| `file-view` / `file-view-path` | Container; path text = the exact `docs/...` path. |
| `file-view-gutter` / `file-view-gutter-line` | Line-number gutter; one `-line` per buffer line (count == `buffer.split("\n").length`). |
| `file-view-editor` | Edit-in-place surface. P17: computed background `rgba(0,0,0,0)` focused AND unfocused, no border/outline/shadow change, no reflow. Monospace font (the scoped exception) while `body` resolves to Poppins. |
| `file-view-flash` | Exact-line jump flash; `+ data-line` (0-based buffer line). Jumps clamp at scroll extremes; cross-page jumps navigate first then scroll (`scroll={false}`). |
| `yaml-invalid-indicator` | Visible when the buffer does not parse (P10b); outline/derived views keep last-valid. |
| `save-state` | `+ data-save-state` ∈ `idle`\|`saving`\|`saved`\|`invalid`\|`conflict`\|`error`; `+ data-revision`. The observable autosave signal (no fixed sleeps). |

### Architecture per-service derived details (on the file page)
| testid | Notes |
|---|---|
| `arch-service-details` | `+ data-service`; the derived details region. Endpoints/tech/provenance are its DESCENDANTS, absent from the other service's region. |
| `service-tech` | `+ data-tech`. |
| `service-endpoint` | `+ data-method`, `+ data-path` (custom services only). |
| `service-connection` | `+ data-key` (supporting/db services only). |
| `service-docker-provenance` | Dockerfile path (custom) or compose_ref (image). |

### Feature aggregation + unaligned bucket
| testid | Notes |
|---|---|
| `feature-description` | The feature's description (from features.yaml). |
| `feature-stories-list` / `feature-scenarios-list` | Derived lists; rows `feature-story-row` (`+ data-story-id`) / `feature-scenario-row` (`+ data-scenario-id`). Re-bucket live on any `feature:` change. |
| `unaligned-bucket` / `unaligned-scenario-row` | Scenarios with no `feature:` OR a dangling ref; `+ data-scenario-id`, `+ data-dangling` (`true`/`false`), `+ data-dangling-ref`. |

### Feature picker (searchable; reused, disambiguated by scoping to its container)
`feature-picker` · `feature-picker-toggle` · `feature-picker-input` ·
`feature-picker-option` (`+ data-feature`) · `feature-picker-create` (inline new).
The list must FILTER as you type (a static `<select>` fails w5.5a).

### New-story form (P22)
`new-story-open` (opens) · `new-story-form` · `new-story-title` ·
`new-story-submit`. The feature picker is **required** (submit blocked without a
feature). Create-new appends an `id`+`description`-only stub to features.yaml and
exclusive-creates the story file.

### Docked chat (P10/P11/P12)
`chat-dock` · `chat-dock-toggle` (edge tab / open) · `chat-dock-panel` (flow
sibling, NOT `position:fixed`; narrows content) · `chat-dock-resizer` ·
`chat-dock-header` · `chat-dock-new` · `chat-dock-history` · `chat-dock-message`
(`+ data-role`) · `chat-dock-input` (pinned at panel bottom) · `chat-dock-send`.
Default width is wide (~40% at 1920). ONE conversation follows navigation.

### Section landings
`home-landing` · `builds-landing` (navigation-only: no `file-view`, no chat edit target).

## Deterministic chat driver (protocol `w6` — no model call)

Enabled by the CLI remote env `TRUE_BDD_CHAT_DRIVER=deterministic` (set by
`WorkspaceEnv`; a10 passes `deterministicChat:false` for the REAL Claude turn).
The driver short-circuits the Claude turn with a scripted structured result
`{reply_text, edit:{path: current_path, new_content}|null}`; the browser applies
the edit to the buffer and the normal debounced `doc_write` persists it (the SAME
S1 path). Directives (schema-aware so the LIVE outline re-derives, not just a
comment append):

- `@probe add-term <name>` — on the architecture file, `new_content` inserts a
  VALID term node (`- name: <name>` + description) under `terms:` → the Terms
  outline gains an `arch-term-row[data-term=<name>]` live.
- `@probe add-scenario <ID>` — on the scenarios file, inserts a VALID scenario
  node `<ID>:` under `scenarios:` → the Scenarios outline gains a
  `scenario-row[data-scenario-id=<ID>]` live.
- `@probe append <text>` — on any file, appends `<text>` as a trailing line
  (persistence-only cases).
- On a NON-file page (Home, feature aggregation — `current_path` null) → EVERY
  directive returns `{reply_text, edit:null}`; NO `doc_write` is issued. The
  assistant reply message (`chat-dock-message[data-role="assistant"]`) is the
  observable "turn finished" signal (tests wait for it before snapshotting).
- The driver must never leak into production UX (gate it behind the env).
