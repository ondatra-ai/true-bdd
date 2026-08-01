# TrueBDD Web Harness

A local, single-user Next.js web interface for driving TrueBDD host projects
from the browser: it shows what's in place and what's missing (documents,
checklists, per-story lifecycle) and runs the TrueBDD commands — `us
create/refine/apply`, `build tests`, `build code` — including the full
interactive `--fix` loop, with prompts answered from the browser.

## Architecture (inverted connection)

The browser and this server **never touch a host project's filesystem**.
Instead, the CLI connects out:

```
host project folder                    harness server              browser
┌─────────────────────┐   HTTP poll   ┌───────────────┐   poll   ┌─────────┐
│ true-bdd remote ────┼──────────────▶│ Next.js + SQLite│◀────────┤ UI      │
│  └─ spawns true-bdd │   events/     │  (127.0.0.1)   │  answers │         │
│     <cmd> as child  │   prompts     └───────────────┘          └─────────┘
└─────────────────────┘
```

- `true-bdd remote` runs **inside** a host project folder, registers with the
  server, uploads an inventory snapshot, polls for dispatched runs, executes
  them as child processes, and relays output/prompts/results.
- "Selecting a folder" in the UI = picking a connected remote session.
  Multiple sessions (folders) can be connected at once.
- Folder-level mutual exclusion is a host-side `flock` (inherited by the
  command child), not server bookkeeping.
- State (sessions, runs, output, prompts, inventory) persists in SQLite
  (`harness/data/harness.db` by default, `TRUE_BDD_HARNESS_DB` to override).
- No cancellation from the browser in v1 — Ctrl+C on the `remote` process is
  the recovery path (runs then report `interrupted`).

## Running

```bash
# 1. Build the CLI (repo root)
mkdir -p ./bin && go build -o ./bin/true-bdd ./src

# 2. Build + start the harness server (production runtime, loopback only)
cd harness && npm ci && npm run build && PORT=4517 npm run start

# 3. In each host project folder you want to drive:
env -u CLAUDECODE /path/to/true-bdd remote --server http://127.0.0.1:4517

# 4. Open http://127.0.0.1:4517
```

Port 4517 is the default; avoid 3000 (used by BDD fixtures). The server
binds `127.0.0.1` and validates Origin/Host on mutating routes — it is not
meant to be exposed beyond the local machine (no auth/TLS).

## Testing

Playwright E2E (fully real — real Go binary, real `claude` calls in the `ai`
project; written before the implementation, per the plan):

```bash
cd harness
npx playwright install chromium   # once

# Protocol suite — no Claude calls, ~1 min total
npx playwright test --project=protocol

# AI suite — REAL Claude calls, ~15–20 min, requires `claude` on PATH
# and authentication; skips loudly when unavailable
npx playwright test --project=ai

# Unit tests (real-SQLite store tests + view-model tables)
npm run test:unit
```

- The suites are local/manual (not a CI merge gate). CI runs lint,
  typecheck, unit tests, and the production build.
- Every E2E test is fully isolated: its own server process, DB, port, and
  writable fixture folder under the repo's `tmp/`. Artifacts and a
  repro manifest are preserved on failure.
- Test fixtures live in `tests/harness/fixtures/` and are materialized by the Go
  helper in `tests/materializer` (repo root), which overlays the live
  engine seed (`true-bdd/` + `templates/`) with designed host-project trees.
- The `data-testid` and API contract used by the tests is documented in
  `tests/harness/helpers/README-testids.md` — it is binding for UI changes.

## Layout

- `app/` — Next.js App Router: pages (`/`, `/sessions/[id]`, `/runs/[id]`),
  API routes (`app/api/`), `lib/` (SQLite store, origin policy, retention),
  `lib/view-model/` (pure render-logic, unit-tested), `components/`.
- `tests/unit/` — vitest: real-SQLite store tests and view-model tables.
  (The Playwright E2E suite moved out to the repo-root `tests/harness/`
  self-contained package — specs, helpers, fixtures, setup/teardown, reporters.)
- `data/` — default runtime SQLite location (gitignored).

The full design (protocol semantics, state machine, retention, security
policy, test inventory) lives in the repo's planning document trail; the
implementation was built test-first against `tmp/harness-plan.md` v5.
