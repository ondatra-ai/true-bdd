import { NextResponse } from "next/server";

import { agentAllowed, forbidden } from "@/app/lib/origin";
import { relayHub } from "@/app/lib/relay/hub";
import { readJsonBody } from "@/app/lib/request-json";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
// Vercel function budget: agent long-poll holds up to 5s (plan r1 #14).
export const maxDuration = 15;

/**
 * POST /api/agent/poll (plan §2). Held ≤5s (an open poll counts as liveness);
 * `wait: false` is the zero-wait raw-protocol variant. 204 on nothing; 200 with
 * a work item bound to the session + epoch; 404 when the registration is stale
 * (unknown session / stale epoch / bad token) so the CLI re-registers with the
 * SAME client-owned session id.
 */
export async function POST(request: Request) {
  if (!agentAllowed(request)) {
    return forbidden();
  }

  const parsed = await readJsonBody(request);
  if (!parsed.ok) {
    return parsed.response;
  }
  const body = parsed.body;

  if (typeof body.session_id !== "string" || typeof body.connection_epoch !== "number") {
    return NextResponse.json({ error: "invalid poll" }, { status: 400 });
  }
  const token = typeof body.capability_token === "string" ? body.capability_token : "";
  const wait = body.wait !== false;

  const result = await relayHub().poll(body.session_id, body.connection_epoch, token, wait);

  if (result.kind === "rejected") {
    return NextResponse.json({ error: result.reason }, { status: 404 });
  }
  if (result.kind === "empty") {
    return new NextResponse(null, { status: 204 });
  }

  return NextResponse.json({
    work_id: result.work_id,
    session_id: body.session_id,
    connection_epoch: body.connection_epoch,
    type: result.type,
    payload: result.payload,
    deadline: Date.now() + 30_000,
  });
}
