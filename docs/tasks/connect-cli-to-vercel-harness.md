<!-- See docs/context/requirements-guide.md. System = architecture/infrastructure
     decisions only ("must use X"); every behavior → Product, from a role. -->
# Connect the true-bdd CLI to the Vercel-deployed harness (Redis-backed relay)

## Goal:
A BDD System Architect should be able to point `true-bdd remote` at the TrueBDD
harness deployed on Vercel and drive a host project from the browser end-to-end —
register a session, dispatch runs, watch output stream, and answer `--fix` prompts
— with the harness's relay coordination backed by Redis so the browser and the CLI
rendezvous across separate serverless invocations.

## Current behavior:
- The harness is a **stateless in-memory relay** (`app/lib/relay/registry.ts`,
  `hub.ts`) — a `globalThis` singleton `RelayHub` that holds **no database**. All
  durable run state lives on the host CLI in `<folder>/tmp/true-bdd-state.db` and
  is streamed back as query replies. (The README / `next.config.ts` still mention
  SQLite, but no SQLite store exists in the shipped code.)
- The relay coordinates browser↔CLI work items via **in-process state and
  Promises**: the browser `await`s the CLI reply *within the same HTTP request*
  (up to 10 s mutation / 30 s inventory), the CLI long-polls ~5 s, a 2 s
  `setInterval` sweep handles expiry, and a **loopback-only Host/Origin policy**
  gates every route. It runs via `next start -H 127.0.0.1`.
- The CLI `remote` command defaults to `http://127.0.0.1:4517` (`--server`
  override) and is otherwise remote-ready: no auth, no Origin/Host headers, default
  TLS, and the prompt/answer path already runs browser→server→child-stdin.
- Vercel project `true-bdd-app` (team `kavoon`) is linked; `REDIS_URL` is
  provisioned in Production/Preview/Development. No Redis code exists yet.

## Requirements

### Product
- [revealed] A BDD System Architect should be able to connect `true-bdd remote` to
  the harness at its Vercel URL (over HTTPS) and have the session appear in the
  browser — pointing the CLI at the URL via `--server` or a set-once env var (the
  default stays local loopback).
- [revealed] A BDD System Architect should be able to dispatch a run (`us
  create/refine/apply`, `build tests`, `build code`) from the browser and have the
  connected CLI execute it on the host, with output appearing **incrementally** in
  the browser as the command runs (not only at completion).
- [suggested] A BDD System Architect should be able to complete a dispatched run
  through the Vercel harness end-to-end even though the browser request and the
  CLI poll are served by separate serverless invocations — the dispatch is
  acknowledged immediately and the browser obtains progress / output / replies
  through later requests, with the run surviving cold starts and concurrent
  instances.
- [suggested] A BDD System Architect should be able to complete the interactive
  `--fix` workflow that creates and verifies a user story entirely from the
  browser: each prompt the running command emits (choice / clarify / freetext)
  appears in the browser chat interface; the Architect submits an answer; that
  answer is delivered to the running command on the host; and the exchange repeats
  until the command finishes.

### System
- [revealed] The true-bdd harness must use Redis as the backend for its relay
  coordination state (session registry, work queue, replies).
- [revealed] The true-bdd harness must deploy on Vercel.
- [revealed] The true-bdd harness must run Redis via a docker-compose file for
  local dev and tests (there is no other backend).

### Harness
- [suggested] A Developer should be able to run the harness's existing unit + E2E
  suites against the Redis-backed relay.

## Non-goals
- No access control — anyone with the Vercel URL can connect a CLI, dispatch runs,
  and read state. [revealed; deliberate]
- Run output, prompts, and inventory are **not** moved to Redis; they remain
  produced and held on the host CLI. The harness relay only coordinates. [prevents
  the "all state in Redis" double-interpretation]
