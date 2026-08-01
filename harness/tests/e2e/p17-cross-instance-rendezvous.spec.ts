/**
 * P17 (plan: connect-cli-to-vercel-harness) — THE serverless proof.
 *
 * Two `next start` instances (A, B) on separate ports share ONE Redis. The
 * test pins register / enqueue / poll / reply to SPECIFIC instances with
 * deterministic routing (Codex r1 #7): a partially-in-process relay MUST NOT
 * pass — every coordination role (registry, work queue, reply, capacity,
 * correlation) is forced across the process boundary.
 *
 * Driven entirely by the raw agent protocol (register/poll/reply) + the
 * browser API — no `true-bdd remote`, no `claude`. Each assertion is phrased
 * to FAIL under the in-process (globalThis-singleton) relay:
 *
 *   1. registry crosses instances — register on B is visible on A (and reverse);
 *   2. work queue + reply cross instances, exact-reply — a browser dispatch on
 *      A is retained as an unresolved promise; poll on B returns that exact
 *      work_id; reply on B; `await pending` resolves 201/ok; a second poll on B
 *      does NOT redeliver the same work_id (consume-once);
 *   3. read query crosses instances — a session_detail read on A is served by
 *      the CLI polling B;
 *   4. correlation crosses instances — a wrong-token reply on B (for a session
 *      registered on A) is rejected as bad_token (409), proving the shared
 *      registry authenticated it; under separate-process relays the session is
 *      unknown on B (410 unknown_session).
 */

import { expect, test } from "@playwright/test";

import {
  newRunToken,
  pollAgent,
  pollUntil,
  registerAgent,
  replyAgent,
  STATUS,
  type AgentRegisterBody,
} from "./helpers/api-client";
import { CrossInstanceEnv } from "./helpers/cross-instance-env";
import { redisExec } from "./helpers/redis";

let env: CrossInstanceEnv | undefined;

test.afterEach(async () => {
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;
  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

function registerBody(sessionId: string, folder: string, pid: number): AgentRegisterBody {
  return {
    session_id: sessionId,
    folder,
    canonical_folder: folder,
    pid,
    start_identity: `p17-${sessionId}`,
    version: "0.0.0-p17",
  };
}

/**
 * Retains a browser request promise so it can be resolved LATER (the
 * retained-promise pattern, Codex r2 #6) without letting a teardown-dropped
 * connection surface an unhandled rejection: if an earlier assertion fails,
 * afterEach stops both servers and any in-flight fetch socket is reset. The
 * no-op `.catch` absorbs that; the real resolution still reaches `await` on
 * the happy path (the promise is fulfilled, not rejected).
 */
function retain<T extends Promise<unknown>>(promise: T): T {
  promise.catch(() => {});
  return promise;
}

test("P17: registry, work queue, reply, and correlation cross next-start instances sharing Redis", async () => {
  // A tiny per-session queue cap (default 64) so Assert 4 can fill it with a
  // couple of reads and prove capacity is enforced cross-instance without
  // holding dozens of concurrent routes. The coder exposes this env knob at the
  // relay config (same pattern as P9's MAX_INVENTORY_REQUEST_BYTES).
  env = await CrossInstanceEnv.start("p17-cross-instance", {
    serverEnv: { HARNESS_MAX_PER_SESSION_QUEUE: "2" },
  });
  const e = env;
  const fixture = await e.materialize("bare-host");
  const folder = fixture.target;

  // ── Assert 1: registry crosses instances (B→A and A→B) ──
  const sessionId = `p17-${newRunToken()}`;
  const regB = await registerAgent(e.portB, registerBody(sessionId, folder, 171717));
  expect(regB.status).toBe(STATUS.ok);
  const creds = JSON.parse(regB.body) as { connection_epoch: number; capability_token: string };

  // The session registered on B is listed by A (separate process ⇒ Redis).
  const onA = await pollUntil(
    async () => (await e.apiA.listSessions()).find((candidate) => candidate.session_id === sessionId),
    { timeoutMs: 15_000, what: "session registered on B to appear in GET /api/sessions on A" },
  );
  expect(onA.pid).toBe(171717);

  // Reverse direction: register on A, list on B.
  const sessionId2 = `p17-${newRunToken()}`;
  const regA = await registerAgent(e.portA, registerBody(sessionId2, folder, 272727));
  expect(regA.status).toBe(STATUS.ok);
  const onB = await pollUntil(
    async () => (await e.apiB.listSessions()).find((candidate) => candidate.session_id === sessionId2),
    { timeoutMs: 15_000, what: "session registered on A to appear in GET /api/sessions on B" },
  );
  expect(onB.pid).toBe(272727);

  // ── Assert 2: work queue + reply cross instances, exact-reply (retained promise) ──
  // RETAIN the browser dispatch as an unresolved promise (Codex r2 #6 — typed
  // helpers await immediately); poll/reply via B, THEN resolve.
  const token = newRunToken();
  const pending = retain(
    e.apiA.dispatchRunResponse(sessionId, {
      command: "version",
      fix: false,
      client_token: token,
    }),
  );

  const pollB = await pollAgent(e.portB, {
    session_id: sessionId,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
  });
  expect(pollB.status).toBe(STATUS.ok);
  const work = JSON.parse(pollB.body) as { work_id: string; type: string; payload: unknown };
  expect(work.type).toBe("dispatch");
  expect((work.payload as { client_token?: string }).client_token).toBe(token);

  // Reply on B with the CLI transactional result; the retained A-side promise
  // resolves to the SAME body (consume-once, exact delivery).
  const replyB = await replyAgent(
    e.portB,
    {
      session_id: sessionId,
      connection_epoch: creds.connection_epoch,
      capability_token: creds.capability_token,
      work_id: work.work_id,
    },
    JSON.stringify({ status: STATUS.created, body: { run_id: "p17-run-1" } }),
  );
  expect(replyB.status).toBe(STATUS.ok);

  const resolved = await pending;
  expect(resolved.status()).toBe(STATUS.created);
  expect((await resolved.json()).run_id).toBe("p17-run-1");

  // Consume-once proof (Codex review r1 #4/#9): the browser's request consumed
  // the reply via GETDEL, so the per-work reply key MUST NOT exist in Redis
  // after the request resolved. Replacing GETDEL with GET (or retaining the
  // reply body for re-service) would leave the key behind — this assertion
  // fails under that regression. The key shape is `<prefix>:reply:<work_id>`.
  const replyKey = `${e.prefix}:reply:${work.work_id}`;
  const afterConsume = await redisExec(["EXISTS", replyKey]);
  expect(afterConsume, `reply key ${replyKey} must be gone after consume-once read`).toBe("0");

  // A second poll on B MUST return 204 no_content — the work was consumed
  // (delivered) and no other work is queued for this session at this point in
  // the test (Codex r1 #7: previously a loose "any different work_id" branch
  // would have let duplicate delivery or queue contamination pass undetected).
  const rePoll = await pollAgent(e.portB, {
    session_id: sessionId,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
    wait: false,
  });
  expect(rePoll.status).toBe(STATUS.no_content);

  // ── Assert 3: a read query crosses instances (A served by CLI polling B) ──
  const readPending = retain(e.apiA.sessionDetailResponse(sessionId));
  const readPoll = await pollAgent(e.portB, {
    session_id: sessionId,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
  });
  expect(readPoll.status).toBe(STATUS.ok);
  const readWork = JSON.parse(readPoll.body) as { work_id: string; type: string };
  expect(readWork.type).toBe("query");

  const detailBody = {
    session_id: sessionId,
    owner_id: sessionId,
    active_run: null,
    runs: [],
    active_owners: [],
    inventory: { snapshot_truncated: false, stories_omitted: 0, limit_too_small: false },
  };
  await replyAgent(
    e.portB,
    {
      session_id: sessionId,
      connection_epoch: creds.connection_epoch,
      capability_token: creds.capability_token,
      work_id: readWork.work_id,
    },
    JSON.stringify({ status: STATUS.ok, body: detailBody }),
  );

  const detail = await readPending;
  expect(detail.status()).toBe(STATUS.ok);
  expect((await detail.json()).session_id).toBe(sessionId);

  // ── Assert 4a: capacity is enforced cross-instance ──
  // Deterministic fill (Codex test-author r2 #1): enqueuing a read does not by
  // itself prove the route reached relay.enqueue before the next request. So
  // enqueue a status read on A and CLAIM it via a raw-agent poll on B — a held
  // poll blocks until the work appears, then moves it queued→delivered, which is
  // the deterministic barrier. DELIVERED work still counts toward the per-session
  // cap (registry counts queued + delivered in-flight), so two claimed reads fill
  // the cap=2 queue with no arrival-order assumption.
  const fillA1 = retain(e.apiA.sessionStatusResponse(sessionId));
  void fillA1; // retained only to occupy queue capacity; never awaited
  const claim1 = await pollAgent(e.portB, {
    session_id: sessionId,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
  });
  expect(claim1.status).toBe(STATUS.ok);
  const fillA2 = retain(e.apiA.sessionStatusResponse(sessionId));
  void fillA2; // retained only to occupy queue capacity; never awaited
  const claim2 = await pollAgent(e.portB, {
    session_id: sessionId,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
  });
  expect(claim2.status).toBe(STATUS.ok);

  // Both claimed reads are DELIVERED (in-flight, counted) → the queue is full. A
  // further status read on A → 503, and on B → 503 too: the shared Redis counter
  // enforces the cap across the instance boundary. Under separate-process relays
  // B cannot see A's in-flight count (its read enqueues/awaits, not 503). The
  // retained reads drain on teardown (retain swallows their eventual 504).
  const overflowA = await e.apiA.sessionStatusResponse(sessionId);
  expect(overflowA.status()).toBe(STATUS.capacity_service);
  const overflowB = await e.apiB.sessionStatusResponse(sessionId);
  expect(overflowB.status()).toBe(STATUS.capacity_service);

  // ── Assert 4b: correlation crosses instances (shared registry authenticates) ──
  // A wrong-token reply on B for a session registered on A: under Redis the
  // shared registry authenticates the session+epoch and rejects the token
  // (bad_token → 409); under separate-process relays the session is unknown on
  // B (unknown_session → 410). The 409 + bad_token body is the discriminator.
  const wrongToken = await replyAgent(
    e.portB,
    {
      session_id: sessionId,
      connection_epoch: creds.connection_epoch,
      capability_token: "not-the-real-token",
      work_id: "p17-no-such-work",
    },
    JSON.stringify({ status: STATUS.ok, body: { ok: true } }),
  );
  expect(wrongToken.status).toBe(STATUS.conflict);
  expect((JSON.parse(wrongToken.body) as { error?: string }).error).toBe("bad_token");
});
