/**
 * P22 (plan: connect-cli-to-vercel-harness) — the delivered→timeout→late-commit
 * recovery path that the consume-once + orphan model exists for (Codex r3 #2;
 * P13 resubmits after SUCCESS, not after a LOST response).
 *
 * Two HONEST sub-proofs, both driven CROSS-INSTANCE (browser on A, CLI on B,
 * one Redis) so the suite is RED on the not-yet-implemented Redis-backed relay
 * (under the in-process relay B never sees A's session/work → 404):
 *
 *   Assert 1 — the relay's orphan contract (raw agent): a dispatch CLAIMED on B
 *     (delivered) is let past the browser's mutation deadline on A → 504; the
 *     CLI's late reply on B is ACCEPTED (the orphan state admits ≤1 late reply);
 *     a SECOND late reply for the SAME work_id is REJECTED 409 (the orphan
 *     retired after one consume — the late body is NOT retained/re-served). No
 *     run_id or inventory is fabricated: this proves the relay's no-store-and-
 *     forward contract, not a CLI commit.
 *
 *   Assert 2 — real CLI commit + token dedup (a REAL remote on B): dispatch a
 *     `version` run with token T on A; the remote on B actually commits it (a
 *     REAL run_id in its store); re-dispatch the SAME token T on A → 200 with the
 *     ORIGINAL run_id (CLI-side dedup), and the project history shows EXACTLY ONE
 *     run — the run the 504'd/late-replied request still committed is recovered
 *     by idempotent re-dispatch, not fabricated. (The plan's late-ANSWER first-
 *     wins case is covered by p20's identical-answer redo.)
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

/**
 * Retains a browser request promise so it can be resolved LATER (the
 * retained-promise pattern, Codex r2 #6) without letting a teardown-dropped
 * connection surface an unhandled rejection: if an earlier assertion fails,
 * afterEach stops the server and the in-flight fetch's socket is reset. The
 * no-op `.catch` absorbs that; the real resolution still reaches `await` on
 * the happy path (the promise is fulfilled, not rejected).
 */
function retain<T extends Promise<unknown>>(promise: T): T {
  promise.catch(() => {});
  return promise;
}

test("P22: a delivered→timeout late reply is consumed once; a real run is recovered by token dedup across instances", async () => {
  env = await CrossInstanceEnv.start("p22-late-reply");
  const e = env;

  // ── Assert 1: the relay's orphan contract (raw agent; folder 1) ──
  const rawFixture = await e.materialize("bare-host");
  const rawFolder = rawFixture.target;
  const rawSession = `p22-${newRunToken()}`;
  const rawBody: AgentRegisterBody = {
    session_id: rawSession,
    folder: rawFolder,
    canonical_folder: rawFolder,
    pid: 222221,
    start_identity: "p22-raw",
    version: "0.0.0-p22",
  };
  const reg = await registerAgent(e.portA, rawBody);
  expect(reg.status).toBe(STATUS.ok);
  const creds = JSON.parse(reg.body) as { connection_epoch: number; capability_token: string };
  const auth = {
    session_id: rawSession,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
  };

  // Dispatch on A (retained); claim on B (delivered). RED under in-process: the
  // poll on B 404s (B does not see A's session).
  const token = newRunToken();
  const pending = retain(e.apiA.dispatchRunResponse(rawSession, { command: "version", fix: false, client_token: token }));
  const claimPoll = await pollAgent(e.portB, auth);
  expect(claimPoll.status).toBe(STATUS.ok);
  const claimed = JSON.parse(claimPoll.body) as { work_id: string; type: string };
  expect(claimed.type).toBe("dispatch");

  // Let the browser dispatch pass its mutation deadline on A ⇒ 504 cli_timeout.
  const timedOut = await pending;
  expect(timedOut.status()).toBe(STATUS.cli_timeout);

  // The CLI POSTS the late reply on B (the run would commit in a real CLI).
  // Under Redis the orphan state accepts ≤1 late reply across the instance
  // boundary. Under separate-process relays B cannot authenticate A's work_id
  // (unknown_session) ⇒ rejected.
  const lateReply = await replyAgent(
    e.portB,
    { ...auth, work_id: claimed.work_id },
    JSON.stringify({ status: STATUS.created, body: { run_id: "p22-late" } }),
  );
  expect(lateReply.status).toBe(STATUS.ok);

  // A SECOND late reply for the SAME work_id is REJECTED 409 — the orphan
  // retired after one consume, so the late body is neither retained nor re-
  // served (no store-and-forward). This is the deterministic no-retain proof.
  const secondLate = await replyAgent(
    e.portB,
    { ...auth, work_id: claimed.work_id },
    JSON.stringify({ status: STATUS.created, body: { run_id: "p22-late-2" } }),
  );
  expect(secondLate.status).toBe(STATUS.conflict);

  // Direct no-retain proof (Codex review r1 #4/#9): the orphan's late reply
  // BODY must NOT be retained for re-service. RESOLVE on the orphaned→late_replied
  // transition only flips the work state — it never writes the reply body to the
  // reply key. The reply key may still hold the SWEEP's session_gone 404 marker
  // (written when the dead session was evicted, restored by FINAL_READ_SCRIPT so
  // it can reach a future request); that marker has its own TTL and is the
  // sweep's notification, not the late reply. So assert the reply key's VALUE
  // is not the late CLI body (a regression that retains the orphan body — e.g.
  // RESOLVE writing the reply key on the orphaned branch — would trip this).
  const replyKey = `${e.prefix}:reply:${claimed.work_id}`;
  const retained = await redisExec(["GET", replyKey]);
  if (retained !== "" && retained !== "(nil)") {
    const parsed = JSON.parse(retained) as { status?: number; body?: { run_id?: string } };
    expect(
      parsed.body?.run_id,
      `orphan must not retain the late CLI reply body for re-service (got ${retained})`,
    ).not.toBe("p22-late");
  }

  // ── Assert 2: real CLI commit + token dedup (a REAL remote on B; folder 2) ──
  // Separate folder so its project store is independent of the raw-agent session
  // above (which committed nothing — there was no real CLI). The remote on B
  // actually executes the run; the re-dispatch recovers it via CLI-side dedup,
  // not a fabricated id.
  const cliFixture = await e.materialize("bare-host");
  const remote = await e.startRemoteOnB(cliFixture.target);
  const session = await pollUntil(
    async () => (await e.apiA.listSessions()).find((candidate) => candidate.pid === remote.pid),
    { timeoutMs: 30_000, what: "the B-registered remote to appear in GET /api/sessions on A" },
  );
  const sessionId = session.session_id;

  // First dispatch of token T → the remote commits the run (a REAL run_id).
  const dedupToken = newRunToken();
  const first = await e.apiA.dispatchRun(sessionId, {
    command: "version",
    fix: false,
    client_token: dedupToken,
  });
  const committedRunId = first.runId;
  await e.apiA.waitForRunTerminal(sessionId, committedRunId);

  // Re-dispatch the SAME token T → 200 with the ORIGINAL run_id (CLI dedup
  // recovers the committed run; 200, not a second 201).
  const second = await e.apiA.dispatchRun(sessionId, {
    command: "version",
    fix: false,
    client_token: dedupToken,
  });
  expect(second.status).toBe(STATUS.ok);
  expect(second.runId).toBe(committedRunId);

  // The project has EXACTLY ONE run — committed once, recovered idempotently.
  // Asserted on the FULL project history (Codex test-author r3): filtering for
  // the already-known run_id would hide a duplicate run the re-dispatch created
  // under a different id (a false proof). The fresh fixture holds exactly one
  // run, and it is the committed id — proven against the REAL CLI store.
  const status = await e.apiA.getSessionStatus(sessionId);
  expect(status.runs.length).toBe(1);
  expect(status.runs[0]?.run_id).toBe(committedRunId);
});
