import { NextResponse } from "next/server";

import { agentAllowed, forbidden } from "@/app/lib/origin";
import { relayHub, RELAY_CONFIG, type Correlation, type CliReply } from "@/app/lib/relay/hub";
import { readJsonBody } from "@/app/lib/request-json";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

/** Extracts the correlation triple from the reply HEADERS (outside the capped
 * body, so an over-cap body still yields a CORRELATED failure). */
function correlationFrom(request: Request): Correlation {
  return {
    sessionId: request.headers.get("x-session-id") ?? "",
    epoch: Number(request.headers.get("x-connection-epoch") ?? "0"),
    token: request.headers.get("x-capability-token") ?? "",
    workId: request.headers.get("x-work-id") ?? "",
  };
}

/** A well-formed reply envelope is an object with a numeric `status` and a
 * `body` field (finding 1: a malformed envelope maps to a correlated 502). */
function isReplyEnvelope(body: Record<string, unknown>): body is { status: number; body: unknown } {
  return typeof body.status === "number" && "body" in body;
}

/** Maps a relay reply-rejection reason to the status returned to the CLI. */
function rejectionStatus(reason?: string): number {
  switch (reason) {
    case "unknown_session":
    case "stale_epoch":
      return 410; // Gone — the CLI's registration is stale; re-register.
    default:
      return 409; // bad_token / unknown_work / not_delivered — CLI-side conflict.
  }
}

/**
 * POST /api/agent/reply (plan §2). Auth/correlation live in HEADERS OUTSIDE the
 * capped body, so an over-cap body still yields a CORRELATED 413 to the browser
 * waiter (never a blind 504). The body is the CLI result `{status, body}`; the
 * relay resolves the awaiting browser Promise by work_id ONLY when the
 * correlation (session + epoch + token) authenticates the DELIVERED waiter.
 * A malformed envelope is a correlated 502; a late reply from a stale epoch or
 * a wrong token is rejected.
 */
export async function POST(request: Request) {
  if (!agentAllowed(request)) {
    return forbidden();
  }

  const correlation = correlationFrom(request);

  const parsed = await readJsonBody(request, RELAY_CONFIG.replyBudgetBytes);
  if (!parsed.ok) {
    // An over-cap body is a CORRELATED failure: authenticate the correlation,
    // then fail the correct browser waiter with a 413 (never unauthenticated).
    if (parsed.response.status === 413) {
      relayHub().failOverCap(correlation);
    }

    return parsed.response;
  }

  if (!isReplyEnvelope(parsed.body)) {
    // Malformed reply envelope → a correlated 502 to the browser waiter.
    relayHub().failInvalid(correlation);

    return NextResponse.json({ error: "invalid_reply" }, { status: 502 });
  }

  const replyBody: CliReply = { status: parsed.body.status, body: parsed.body.body };

  const result = relayHub().reply(correlation, 0, replyBody);
  if (!result.ok) {
    return NextResponse.json({ error: result.reason }, { status: rejectionStatus(result.reason) });
  }

  return NextResponse.json({ ok: true });
}
