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
