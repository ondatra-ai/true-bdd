/**
 * P18 (plan: connect-cli-to-vercel-harness) — cold-start survival.
 *
 * One server, one Redis. Uses a raw registered agent that does NOT poll
 * (Codex r1 #8 — a live CLI would claim work before the restart and mask
 * nondurability). Under the in-process relay a restart WIPES the registry and
 * the work queue (process memory gone); under Redis both survive. Each
 * assertion fails under the in-process model:
 *
 *   1. registry survives a memory wipe — register returns connection_epoch +
 *      capability_token; after server.restart() the session is STILL listed
 *      (same session_id + pid) WITHOUT a re-register, and a raw-agent poll with
 *      the SAME epoch+token SUCCEEDS (the epoch/token are not exposed by the
 *      browser session summary, so they are asserted via the raw agent path —
 *      Codex r2 #9). In-process ⇒ the registry is wiped ⇒ 404 ⇒ the poll is
 *      rejected and a NEW epoch would be minted on re-register.
 *   2. queued work survives — START the browser dispatch on the pre-restart
 *      server but do NOT let any agent poll; restart BEFORE any poll (the
 *      work_id is opaque pre-poll — Codex r2 #5); the agent poll AFTER restart
 *      returns the pre-restart dispatch PAYLOAD (command + client_token),
 *      delivered ONCE. In-memory ⇒ the queued work dies with the process.
 */

import { expect, test } from "@playwright/test";

import { newRunToken, pollAgent, pollUntil, registerAgent, replyAgent, STATUS, type AgentRegisterBody } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";
import { waitForQueuedWork } from "./helpers/redis";

let env: ProtocolEnv | undefined;

test.afterEach(async () => {
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;
  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

test("P18: a relay restart keeps the registry and queued work (Redis-backed coordination state)", async () => {
  env = await ProtocolEnv.start("p18-relay-restart");
  const e = env;
  const fixture = await e.materialize("bare-host");
  const folder = fixture.target;
  const port = e.server.port;

  // ── Assert 1: registry + credentials survive a memory wipe ──
  const sessionId = `p18-${newRunToken()}`;
  const body: AgentRegisterBody = {
    session_id: sessionId,
    folder,
    canonical_folder: folder,
    pid: 181818,
    start_identity: "p18-identity",
    version: "0.0.0-p18",
  };
  const reg = await registerAgent(port, body);
  expect(reg.status).toBe(STATUS.ok);
  const creds = JSON.parse(reg.body) as { connection_epoch: number; capability_token: string };

  await e.server.restart();

  // The session is STILL listed immediately after the restart — no re-register
  // wait. Under the in-process model the registry is wiped → the list omits it.
  const afterRestart = await pollUntil(
    async () => (await e.api.listSessions()).find((candidate) => candidate.session_id === sessionId),
    { timeoutMs: 15_000, what: "the session to remain listed across a relay restart" },
  );
  expect(afterRestart.session_id).toBe(sessionId);
  expect(afterRestart.pid).toBe(181818);

  // Epoch/token are persisted in Redis (not exposed by the browser summary) —
  // a poll with the SAME creds SUCCEEDS on the restarted server. In-process ⇒
  // the registry is wiped ⇒ the poll is rejected (unknown_session) and a fresh
  // register would mint a NEW epoch.
  const postRestartPoll = await pollAgent(e.server.port, {
    session_id: sessionId,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
    wait: false,
  });
  expect(postRestartPoll.status).toBe(STATUS.no_content);

  // ── Assert 2: queued work survives (no pre-restart ID capture, no poll) ──
  const token = newRunToken();
  // START the browser dispatch on the pre-restart server but do NOT let any
  // agent poll. The fetch connection drops when the server stops — swallow it.
  const dispatchPromise = e.api
    .dispatchRunResponse(sessionId, { command: "version", fix: false, client_token: token })
    .catch(() => null);

  // Deterministic enqueue barrier (Codex r1 #6): poll Redis directly until the
  // work queue for this session is non-empty, then restart. The previous
  // `sleep(1000)` was the least-coupling barrier available BEFORE the Redis key
  // scheme existed; the scheme is now stable (`<prefix>:workq:<sid>` LIST), so
  // a Redis-side wait is both deterministic and stable. No agent poll is
  // issued (which would CLAIM the work — this assertion forbids that).
  await waitForQueuedWork(e.prefix, sessionId);
  await e.server.restart();

  // The agent poll AFTER restart returns the pre-restart dispatch payload,
  // delivered ONCE. In-memory ⇒ the queued work died with the old process ⇒
  // the poll is rejected unknown_session (the registry was wiped too).
  const workPoll = await pollAgent(e.server.port, {
    session_id: sessionId,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
  });
  expect(workPoll.status).toBe(STATUS.ok);
  const work = JSON.parse(workPoll.body) as { work_id: string; type: string; payload: { command?: string; client_token?: string } };
  expect(work.type).toBe("dispatch");
  expect(work.payload.client_token).toBe(token);
  expect(work.payload.command).toBe("version");

  // Reply on the restarted server; a second poll does NOT redeliver (once).
  const reply = await replyAgent(
    e.server.port,
    {
      session_id: sessionId,
      connection_epoch: creds.connection_epoch,
      capability_token: creds.capability_token,
      work_id: work.work_id,
    },
    JSON.stringify({ status: STATUS.created, body: { run_id: "p18-run-1" } }),
  );
  expect(reply.status).toBe(STATUS.ok);

  const rePoll = await pollAgent(e.server.port, {
    session_id: sessionId,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
    wait: false,
  });
  expect(rePoll.status).toBe(STATUS.no_content);

  // The pre-restart dispatch fetch dropped with the old process; settle it.
  await dispatchPromise;
});
