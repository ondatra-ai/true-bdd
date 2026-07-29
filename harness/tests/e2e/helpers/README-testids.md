# Harness UI + API contract (defined by the protocol specs)

The protocol Playwright specs (`p1-*.spec.ts` … `p8-*.spec.ts`) were
authored BEFORE any feature code (plan §4.3, test-first). This file is
the contract the server and UI phases implement to. The typed source
of truth is `helpers/api-client.ts` (routes, bodies, status codes) and
`helpers/ui.ts` (routes, testids); this document is the human summary.

## Navigation

| Route | View |
|---|---|
| `/` | Sessions list |
| `/sessions/<session-id>` | Session detail (inventory, epics, stories, actions, run history) |
| `/runs/<run-id>` | Run view (output tail, prompt panel, outcome badge) |

All three views **live-poll** their API and re-render without a manual
reload. Tests assert with locator auto-wait against a self-updating
page (e.g. the run-outcome badge flips to `abandoned` after clicking
"Mark abandoned" with no reload).

## API surface (browser-facing)

| Route | Success | Errors |
|---|---|---|
| `GET /api/sessions` | 200 `{sessions: SessionSummary[]}` | 403 wrong Host |
| `GET /api/sessions/:id` | 200 `SessionDetail` | 404 |
| `POST /api/sessions/:id/runs` `{command, story_id?, fix, client_token}` | 201 `{run_id}`; 200 `{run_id}` on client_token dedup (same id) | 400 invalid command/body; 409 session unreachable OR any non-terminal run on the session; 403 origin/host |
| `POST /api/sessions/:id/refresh` | 202 | 403/404 |
| `GET /api/runs/:id?after_seq=N` | 200 `RunDetail` | 404 |
| `POST /api/runs/:id/answer` `{prompt_id, value}` | 200 (accepted or exact retry) | 409 conflicting retry; 400 invalid |
| `POST /api/runs/:id/abandon` | 200 (run → terminal `abandoned`) | 409 not eligible (session reachable or run terminal) |

- `command` enum (exact strings): `version`, `us-create`, `us-refine`,
  `us-apply`, `build-tests`, `build-code`. Anything else → 400.
- `SessionSummary`: `id`, `folder` (canonical **realpath** of the
  remote's cwd — never a symlink path), `pid` (the remote's pid,
  disambiguates same-folder sessions), `reachability`
  (`connected|unreachable`), `active_run_id`, `inventory_generation`
  (0 until the first promoted snapshot), `inventory_updated_at`
  (wall-clock ms of the last promoted inventory, `null` until one exists
  — finding 8).
- `SessionDetail` = summary + `runs: RunSummary[]` (history, newest
  first, survives server restarts — same DB).
- Run `state`: `queued → claimed → running ⇄ prompt_published →
  answer_accepted → answer_consumed → terminal`. `outcome` is `null`
  until terminal, then one of `ok`, `converged`, `not_fixed`,
  `user_exit`, `max_attempts`, `interrupted`, `abandoned`, `error`
  (with `error_detail`: `spawn|no_result|contradiction|folder_locked`).
- `RunSummary`/`RunDetail` additionally carry (finding 7/8, additive):
  `created_at`, `updated_at` (wall-clock ms), and `envelope`, the full
  terminal envelope `{classification, engine_outcome, finalization_ok,
  exit_code, signal}` (all fields `null` until terminal). The badge stays
  `outcome`/`classification`; the other envelope fields are diagnostics.
  The agent wire (`POST /api/agent/events` terminal event) carries the
  same `engine_outcome`, `finalization_ok`, `exit_code`, `signal`, plus a
  new `lock_acquired` event type (no payload — the authoritative flock
  proof, finding 5).

## Origin + Host policy (plan §3.8, asserted by P6)

- Browser mutation routes (`POST /api/sessions/:id/runs`, `/refresh`,
  `/api/runs/:id/answer`, `/abandon`): require the exact allowed
  Origin (`http://127.0.0.1:<port>`) AND Host; missing Origin,
  `Origin: null`, foreign Origin, or wrong Host → 403.
- Agent routes (`/api/agent/*`): loopback Host required; any PRESENT
  foreign Origin → 403; absent Origin is accepted.
- GETs: Host check only → 403 on wrong Host.
- 403 is decided BEFORE business logic; an accepted request may then
  fail with 4xx business errors but never 403.

## Behavioral constants the tests rely on

- **Reachability threshold**: a session with no agent poll for the
  threshold flips to `unreachable`. Default must be between 5 s and
  15 s (recommended 10 s) — P2/P5 dispatch "while still connected"
  immediately after SIGSTOP and poll the flip with a 60 s ceiling.
  Reachability NEVER changes run state.
- **`version` run output**: the run view output must contain
  `true-bdd version <something>` (asserted as `/true-bdd version \S+/`)
  and the run terminates with outcome `ok`.
- **Re-inventory after every command**: a terminal run bumps the
  session's `inventory_generation` (P8 asserts strictly-greater).
- Disabled controls are real `disabled` buttons (Playwright
  `toBeDisabled()`); actions are disabled ONLY for the session's own
  active run or an unreachable session — sibling-session folder
  activity shows the warning banner with controls ENABLED (P5).

## data-testid contract

Attribute conventions: dynamic state goes into `data-*` attributes
(exact strings below), display text is asserted only where noted.

### Sessions list (`/`)

| testid | Notes |
|---|---|
| `session-row` | One per session; carries `data-session-id`, `data-folder`. |
| `session-folder` | Text = canonical folder (realpath). Inside the row. |
| `session-reachability` | Text exactly `connected` or `unreachable`. Inside the row AND on session detail. |
| `session-inventory-age` | Compact promoted-inventory age; `data-stale` ∈ `true`, `false` (finding 8). |
| `test-connection` | Per-row control; dispatches a `version` run for that session (P8). |

### Session detail (`/sessions/<id>`)

| testid | Notes |
|---|---|
| `inventory-generation` | Text = promoted generation number. |
| `inventory-updated-at` | Promoted-inventory age (text) with `data-stale` ∈ `true`, `false` (finding 8). |
| `refresh` | Triggers `POST /api/sessions/:id/refresh` (P3). |
| `folder-warning-banner` | Visible when a SIBLING session on the same folder has a non-terminal run (P5). |
| `path-mismatch-warning` | Visible when the configured architecture path differs from the canonical default (P4). |
| `run-history` / `run-row` | `run-row` carries `data-run-id`, `data-command` (P7). |
| `action-build-tests`, `action-build-code` | Session-level `<button>`s; `disabled` per the rules above (P5). |
| `action-create`, `action-refine`, `action-apply`, `fix-toggle` | Per-story-row action controls (AI specs). |
| `inventory-doc-<key>` | Status chip; `data-status` ∈ `present`, `missing`, `invalid`, `not_a_dir`, `present_empty`, `ambiguous`, `unknown`. Keys: `config` (`true-bdd/true-bdd.yaml`), `prd`, `architecture`, `registry` (`docs/scenarios.yaml`), `stories-dir`, `epics-dir`, `checklist-<stem>` for stems `us-create`, `us-refine`, `us-apply`, `build-tests`, `build-code`. |
| `inventory-truncated-banner` | Visible on the session page when the promoted snapshot was degraded to fit the request budget — `snapshot_truncated` is true or any omission count (`stories_omitted`, per-story `raw_omitted`/`content_omitted`) is positive (plan §1a). Names the degrade mode(s) reached. |
| `epic-section` | Accordion wrapper around one epic's `epic-row` header plus its story-row panel. Carries `data-epic-file` (basename) and `data-epic-number`. UI/modal keying uses `(epic.file, position)`, NOT the create id — duplicate epic numbers declare colliding position-derived ids, so story-row lookups MUST be scoped to the section. Epics render DEFAULT-EXPANDED (collapse is a user action). |
| `epic-toggle` | Expand/collapse control inside the section header; carries `aria-expanded` ∈ `true`, `false`. Rendered ONLY when a story panel exists — invalid epics (no rows) and noncanonical-filename epics (deliberately no rows) render the header + flags WITHOUT a toggle. Collapsing hides the story rows; re-expanding restores them. |
| `epic-title` | Text = `Epic.Title` (the epic document's `name:`), falling back to the filename when unparseable. |
| `epic-row` | Carries `data-epic-file` (basename), `data-epic-number` (integer parsed from the `epic-NN-*` filename, no leading zeros), `data-status` ∈ `parseable`, `invalid`. Lives inside its `epic-section`. |
| `epic-flag-duplicate-number` | Inside every epic row sharing a filename number. |
| `epic-flag-id-mismatch` | Inside an epic row whose document `epic.id` differs from the filename number. |
| `epic-flag-noncanonical-filename` | Inside an epic row whose filename is not the canonical `epic-%02d-*` encoding `us create` resolves (finding 4). Such an epic carries NO Create-addressable story rows (its snapshot `stories` is empty), so its section has NO `epic-toggle`. |
| `story-row-<create-id>` | One per epic story row; `<create-id>` is the position-derived `<epic-filename-number>.<position>` (e.g. `42.1`). Carries `data-epic-number`, `data-position`, `data-declared-id` (epic-declared story id), `data-file-id` (story-file internal id, when resolvable). Lives inside its `epic-section`'s story panel. |
| `story-title` | ALWAYS a `<button>` for a declared story row — clicking opens the story review modal (plan §1b guarantees file content, epic-declared content, or an identity-only fallback exists). Ambiguous rows additionally carry `data-match-count` and show the count inline. |
| `story-created` | Cell inside the story row; `data-status` ∈ `one`, `missing`, `ambiguous`, `invalid`. |
| `story-applied` | Cell; countable → text exactly `x/y` with `data-status="counted"`, `data-applied`, `data-total`; not countable → text `unknown (<reason>)` with `data-status="unknown"`, `data-reason` ∈ `missing`, `ambiguous`, `invalid`, `deprecated_format`, `no_acceptance_criteria`, `empty_internal_id`, `registry_missing`, `registry_invalid`. Counting: registry entries whose `user_stories[]` reference the EXACT story path and the position-derived lineage id `<internal-id>-NNN` (`%03d`). A missing/unparseable registry taints an otherwise-eligible story `unknown(registry_missing|registry_invalid)` — a valid empty `scenarios: {}` stays counted `0/y` (finding 4). |
| `story-refined` | Cell; text exactly `not recorded` in v1. |
| `story-flag-duplicate-declared-id`, `story-flag-id-mismatch`, `story-flag-deprecated-format`, `story-flag-no-acs`, `story-flag-empty-internal-id` | Flag chips inside the story row, visible when the condition holds. |

### Story review modal (`/sessions/<id>`, opened from `story-title`)

Opened via a native `<dialog>` + `showModal()` — native modality gives
focus containment, Escape-to-close, and a backdrop; there is no
hand-written focus trap. Content freshness follows the last scan; on a
generation change the open modal's story is re-derived by composite
identity `(epic.file, position)`.

| testid | Notes |
|---|---|
| `story-modal` | The `<dialog>`. Carries `data-story-id` (the position-derived create id). `aria-labelledby` points at `story-modal-title`. |
| `story-modal-panel` | The single inner content panel. A click on the panel must NOT close the modal; a click on the backdrop (outside the panel) closes it. |
| `story-modal-title` | The modal heading; accessible name = story id + title. |
| `story-modal-close` | Labelled close button → closes the modal and restores focus to the opener (`story-title`). |
| `story-modal-status` | Status chip; falls back to `declared_status`, then `unknown`. |
| `story-modal-created` / `story-modal-applied` / `story-modal-refined` | Lifecycle chips mirroring the row's created / applied / refined cells. |
| `story-modal-identity` | Identity line (declared id / file id). |
| `story-modal-tablist` | `role="tablist"`; keyboard-navigable. |
| `story-modal-tab-review` / `story-modal-tab-raw` | `role="tab"` with `aria-selected` ∈ `true`, `false`. Review is default. Review is enabled IFF declared content exists (file content or epic-declared content); otherwise `aria-disabled`. An invalid story opens on Raw. |
| `story-modal-panel-review` / `story-modal-panel-raw` | `role="tabpanel"` for each tab. |
| `story-modal-statement` | The as-a / i-want / so-that statement block (from file content, else epic-declared content). |
| `story-modal-ac` | One per acceptance criterion; carries `data-ac-id` (the AC id); contains its description. |
| `story-modal-step` | One per Gherkin step; carries `data-kind` ∈ `given`, `when`, `then`, `and`, `but`; its text is the exact step text. Source order preserved. |
| `story-modal-raw` | The verbatim story file in a scrollable `<pre>` (Raw tab). `textContent` equals the file bytes exactly (a truncation/omission notice, when present, is a sibling `story-modal-notice`). |
| `story-modal-error` | Parse-error banner shown when the story file is invalid (modal opens on Raw). |
| `story-modal-notice` | Availability / omission / freshness notice; carries `data-reason` ∈ `not_created` (missing file), `ambiguous` (+ match count), `changed_on_disk`, `raw_truncated`, `raw_omitted`, `content_omitted`, `both_omitted`, `invalid_without_raw`. |

### Run view (`/runs/<id>`)

| testid | Notes |
|---|---|
| `run-state` | Text = current run state (e.g. `queued`, `terminal`). |
| `run-outcome` | Text = outcome badge (the terminal envelope CLASSIFICATION): `ok`, `converged`, `not_fixed`, `user_exit`, `max_attempts`, `interrupted`, `abandoned`, or `error(<detail>)`. |
| `run-elapsed` / `run-last-activity` | Run elapsed time and time-since-last-activity (finding 8). |
| `run-envelope-engine-outcome`, `run-envelope-finalization`, `run-envelope-exit-code`, `run-envelope-signal` | The full terminal-envelope diagnostics beyond the badge (finding 7); present only once terminal and only when the corresponding fact is set. |
| `run-output` | Output tail (contains `true-bdd version …` for version runs). |
| `reachability-overlay` | Visible while the owning session is unreachable and the run is non-terminal (P2). |
| `mark-abandoned` | Control (unreachable session + non-terminal run) → `POST /api/runs/:id/abandon` (P2). |
| `prompt-panel` | Visible while a prompt is pending. Carries `data-kind` ∈ `choice`, `clarify`, `freetext` and `data-prompt-id` (the child-emitted prompt id — A2 asserts the two choice prompts have DISTINCT ids). |
| `prompt-choice-apply`, `prompt-choice-refine`, `prompt-choice-exit` | Choice-prompt buttons (`kind="choice"`); apply/refine/exit map to the engine's `UserAction` (AI specs A0/A2/A3). |
| `prompt-answer-input`, `prompt-answer-submit` | Single-line clarify answer control (`kind="clarify"`): the value is the option NUMBER; the engine collector maps it to the option text (A3). |
| `prompt-clarify-option` | One chip per numbered clarify option; carries `data-index` (1-based) with text = the exact option string. A3 reads the option text at the answered index and asserts the recorded answer equals it. |
| `prompt-freetext-input`, `prompt-freetext-submit` | MULTILINE `<textarea>` refinement-feedback control (`kind="freetext"`, A2). Distinct from the single-line clarify input so the multiline transport (incl. the terminating blank line the remote appends before `AskRefinementFeedback` returns) is exercised end-to-end. |

**AI-call budget (AI specs, NOT a UI testid):** every A-test counts the
`claude` processes spawned as descendants of its remote (polling `ps`
for the command child's subtree — `helpers/claude-budget.ts`) and fails
if a single-check fixture exceeds a small per-test bound. This catches a
broken checklist filter (which would walk the full checklist and spawn
one evaluation call per prompt) before it becomes a multi-hour run. It is
a process-level oracle, deliberately not a rendered surface.
