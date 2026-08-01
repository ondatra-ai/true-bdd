/**
 * P19 (plan: connect-cli-to-vercel-harness) — Vercel-readiness of the origin policy.
 *
 * Deployed mode is ENV-driven (`HARNESS_DEPLOYED=1`), NEVER inferred from the
 * request Host (Codex r1 #12 — Host-inference is spoofable and would defeat
 * loopback CSRF protection on local dev). The effective public host for the
 * browser-mutation CSRF guard is derived from trusted Vercel forwarded headers
 * (`x-forwarded-host` / `x-forwarded-proto`) or `HARNESS_PUBLIC_ORIGIN`, NOT the
 * raw `Host` (Codex r2 #12 — raw-Host compare breaks behind proxies and on
 * preview/custom/alias domains).
 *
 * Two servers: a DEPLOYED one (`HARNESS_DEPLOYED=1`) and a LOOPBACK one (no
 * env). Raw HTTP drives every route family with a non-loopback Host. Under the
 * current loopback-only policy each non-loopback request is 403; under deployed
 * mode GETs/agents are admitted and mutations admit an Origin matching the
 * EFFECTIVE host (foreign / null / missing Origin still 403). The spoof guard
 * on the loopback server stays 403, proving deployed mode is Host-inference-free.
 */

import { expect, test } from "@playwright/test";

import { newRunToken, pollAgent, rawHttpRequest, registerAgent, replyAgent, STATUS } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";

let deployedEnv: ProtocolEnv | undefined;
let loopbackEnv: ProtocolEnv | undefined;

test.afterEach(async () => {
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const deployed = deployedEnv;
  const loopback = loopbackEnv;
  deployedEnv = undefined;
  loopbackEnv = undefined;
  if (deployed !== undefined) {
    await deployed.teardown(test.info());
  }
  if (loopback !== undefined) {
    await loopback.teardown(test.info());
  }
});

const DEPLOYED_HOST = "true-bdd-app.vercel.app";
const JSON_HEADERS = { "content-type": "application/json" };

test("P19: HARNESS_DEPLOYED admits non-loopback Host on GET/agent + effective-host mutation Origin; spoof guard kept", async () => {
  deployedEnv = await ProtocolEnv.start("p19-deployed", { serverEnv: { HARNESS_DEPLOYED: "1" } });
  loopbackEnv = await ProtocolEnv.start("p19-loopback");
  const dport = deployedEnv.server.port;
  const lport = loopbackEnv.server.port;

  // ── GET family: non-loopback Host → 200 in deployed mode (403 today) ──
  const getDeployed = await rawHttpRequest({
    port: dport,
    method: "GET",
    path: "/api/sessions",
    headers: { host: DEPLOYED_HOST },
  });
  expect(getDeployed.status).toBe(STATUS.ok);

  // Every browser GET route family must clear the deployed-mode origin gate,
  // not just the session list. Cover session detail + run detail (Codex review
  // r1 #6): both run a `hasSession` check after the gate, so a non-existent
  // session surfaces exactly 404 session_gone — proving the request reached
  // business logic (not a 403 origin block, not a loose 400/500). The runs-path
  // uses a real run_id shape to exercise the run_detail route specifically.
  const sessionDetailPath = `/api/sessions/p19-nope-${newRunToken()}`;
  const sessionDetail = await rawHttpRequest({
    port: dport,
    method: "GET",
    path: sessionDetailPath,
    headers: { host: DEPLOYED_HOST },
  });
  expect(sessionDetail.status).toBe(STATUS.session_gone);
  expect((JSON.parse(sessionDetail.body) as { error?: string }).error).toBe("session_gone");

  const runDetailPath = `${sessionDetailPath}/runs/p19-no-run-${newRunToken()}`;
  const runDetail = await rawHttpRequest({
    port: dport,
    method: "GET",
    path: runDetailPath,
    headers: { host: DEPLOYED_HOST },
  });
  expect(runDetail.status).toBe(STATUS.session_gone);

  // ── Agent register/poll/reply: ABSENT Origin + non-loopback Host → accepted ──
  // (the Go client sends no Origin). Every agent route family must clear the
  // deployed-mode origin gate — not just register.
  const sessionId = `p19-${newRunToken()}`;
  const regBody = {
    session_id: sessionId,
    folder: "/tmp/p19",
    canonical_folder: "/tmp/p19",
    pid: 191919,
    start_identity: "p19-identity",
    version: "0.0.0-p19",
  };
  const reg = await registerAgent(dport, regBody, { host: DEPLOYED_HOST });
  expect(reg.status).toBe(STATUS.ok);
  const creds = JSON.parse(reg.body) as { connection_epoch: number; capability_token: string };
  const agentHeaders = { host: DEPLOYED_HOST };

  // Poll with ABSENT Origin + deployed Host → 204 nothing (accepted-class; no
  // work is queued). 403 under the loopback-only policy.
  const pollRes = await pollAgent(
    dport,
    {
      session_id: sessionId,
      connection_epoch: creds.connection_epoch,
      capability_token: creds.capability_token,
      wait: false,
    },
    agentHeaders,
  );
  expect(pollRes.status).toBe(STATUS.no_content);

  // Reply with ABSENT Origin + deployed Host → 409 unknown_work (Codex test-author
  // r2 #2: a deterministic 409, not a loose not-403). The route clears the origin
  // gate, authenticates the valid session+epoch+token, then rejects the unknown
  // work_id on its own merits — proving BOTH that Origin was not the blocker AND
  // that the request reached the correlation logic. 403 under the loopback policy.
  const replyRes = await replyAgent(
    dport,
    {
      session_id: sessionId,
      connection_epoch: creds.connection_epoch,
      capability_token: creds.capability_token,
      work_id: "p19-no-such-work",
    },
    JSON.stringify({ status: STATUS.ok, body: {} }),
    agentHeaders,
  );
  expect(replyRes.status).toBe(STATUS.conflict);

  // ── Browser mutation: Origin matching the EFFECTIVE host → accepted-class ──
  // Origin check fires BEFORE hasSession, so a non-existent session surfaces the
  // gate result (404 accepted-class vs 403 blocked) without a 10 s enqueue wait.
  const runsPath = `/api/sessions/p19-nope-${newRunToken()}/runs`;
  const dispatchBody = JSON.stringify({ command: "version", fix: false, client_token: newRunToken() });

  // Raw Host is NOT authoritative in deployed mode (Codex r2 #12): a mutation
  // carrying a spoofed non-loopback Host and a MATCHING Origin to that spoofed
  // Host — but NO trusted forwarded headers and no HARNESS_PUBLIC_ORIGIN — is
  // REJECTED 403. The effective host is undefined, so no Origin can match it;
  // this is the guard against raw-Host inference. Admission via the EFFECTIVE
  // host (forwarded headers) is covered by the case below.
  const rawHostSpoof = await rawHttpRequest({
    port: dport,
    method: "POST",
    path: runsPath,
    headers: { ...JSON_HEADERS, host: DEPLOYED_HOST, origin: `https://${DEPLOYED_HOST}` },
    body: dispatchBody,
  });
  expect(rawHostSpoof.status).toBe(STATUS.forbidden);

  // Foreign Origin → 403 (CSRF guard kept in deployed mode too).
  const foreignOrigin = await rawHttpRequest({
    port: dport,
    method: "POST",
    path: runsPath,
    headers: { ...JSON_HEADERS, host: DEPLOYED_HOST, origin: "http://evil.example" },
    body: dispatchBody,
  });
  expect(foreignOrigin.status).toBe(STATUS.forbidden);

  // Origin: null → 403.
  const nullOrigin = await rawHttpRequest({
    port: dport,
    method: "POST",
    path: runsPath,
    headers: { ...JSON_HEADERS, host: DEPLOYED_HOST, origin: "null" },
    body: dispatchBody,
  });
  expect(nullOrigin.status).toBe(STATUS.forbidden);

  // Missing Origin → 403 (mutations require Origin).
  const missingOrigin = await rawHttpRequest({
    port: dport,
    method: "POST",
    path: runsPath,
    headers: { ...JSON_HEADERS, host: DEPLOYED_HOST },
    body: dispatchBody,
  });
  expect(missingOrigin.status).toBe(STATUS.forbidden);

  // The answer mutation route (a SEPARATE mutation family — Codex review r1 #6)
  // must enforce the same CSRF guard: a foreign Origin on the answer path is
  // 403 in deployed mode. Uses a non-existent session so the origin gate fires
  // before any business logic; the discriminator is 403 (origin blocked) vs
  // 404 (origin admitted, session_gone from hasSession).
  const answerPath = `${runsPath}/p19-no-run/answer`;
  const answerBody = JSON.stringify({ prompt_id: "p19-no-prompt", value: "1" });
  const foreignAnswer = await rawHttpRequest({
    port: dport,
    method: "POST",
    path: answerPath,
    headers: { ...JSON_HEADERS, host: DEPLOYED_HOST, origin: "http://evil.example" },
    body: answerBody,
  });
  expect(foreignAnswer.status).toBe(STATUS.forbidden);

  // ── Effective host from trusted forwarded headers (not raw Host) ──
  // Behind the Vercel proxy the raw Host is loopback but the effective public
  // host is x-forwarded-host. Origin matching the FORWARDED host is accepted;
  // a raw-Host compare would reject this (Codex r2 #12). Deterministic
  // accepted-class outcome (Codex r1 #8): the session doesn't exist, so the
  // route clears the origin gate, the hasSession check fires, and the response
  // is exactly 404 session_gone — proving the request reached business logic
  // (not 403 origin block, not a loose 400/500).
  const forwarded = await rawHttpRequest({
    port: dport,
    method: "POST",
    path: runsPath,
    headers: {
      ...JSON_HEADERS,
      host: `127.0.0.1:${dport}`,
      "x-forwarded-host": DEPLOYED_HOST,
      "x-forwarded-proto": "https",
      origin: `https://${DEPLOYED_HOST}`,
    },
    body: dispatchBody,
  });
  expect(forwarded.status).toBe(STATUS.session_gone);
  expect((JSON.parse(forwarded.body) as { error?: string }).error).toBe("session_gone");

  // Hardened forwarded-header validation (Codex r1 #4): a comma-list
  // x-forwarded-host and a non-http(s) x-forwarded-proto must NOT establish an
  // effective origin — every mutation carrying such headers is rejected 403
  // (no effective host can be derived, so no Origin can match).
  const commaList = await rawHttpRequest({
    port: dport,
    method: "POST",
    path: runsPath,
    headers: {
      ...JSON_HEADERS,
      host: `127.0.0.1:${dport}`,
      "x-forwarded-host": `${DEPLOYED_HOST}, evil.example`,
      "x-forwarded-proto": "https",
      origin: `https://${DEPLOYED_HOST}`,
    },
    body: dispatchBody,
  });
  expect(commaList.status).toBe(STATUS.forbidden);

  const badProto = await rawHttpRequest({
    port: dport,
    method: "POST",
    path: runsPath,
    headers: {
      ...JSON_HEADERS,
      host: `127.0.0.1:${dport}`,
      "x-forwarded-host": DEPLOYED_HOST,
      "x-forwarded-proto": "file",
      origin: `https://${DEPLOYED_HOST}`,
    },
    body: dispatchBody,
  });
  expect(badProto.status).toBe(STATUS.forbidden);

  // ── Spoof guard: a LOOPBACK server (no HARNESS_DEPLOYED) rejects a spoofed Host ──
  // Deployed mode must NOT be inferable from the Host header — otherwise a local
  // dev server could be fooled by a spoofed Vercel Host. Stays 403 on loopback.
  const spoof = await rawHttpRequest({
    port: lport,
    method: "GET",
    path: "/api/sessions",
    headers: { host: DEPLOYED_HOST },
  });
  expect(spoof.status).toBe(STATUS.forbidden);
});
