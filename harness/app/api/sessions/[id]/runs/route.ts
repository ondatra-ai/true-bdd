import { NextResponse } from "next/server";

import { browserMutationAllowed, forbidden } from "@/app/lib/origin";
import { relayHub, DEADLINE_MS } from "@/app/lib/relay/hub";
import { readJsonBody } from "@/app/lib/request-json";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

/**
 * POST /api/sessions/:id/runs (plan §3) — a relayed dispatch mutation. The CLI
 * store transaction (not the relay) decides created (201) / deduped (200) /
 * conflict (409) / invalid (400); the relay just carries the reply status
 * through. A frozen CLI never commits, so the browser sees 504 cli_timeout.
 */
export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  if (!browserMutationAllowed(request)) {
    return forbidden();
  }

  const { id } = await params;

  const parsed = await readJsonBody(request);
  if (!parsed.ok) {
    return parsed.response;
  }

  if (!relayHub().hasSession(id)) {
    return NextResponse.json({ error: "session_gone" }, { status: 404 });
  }

  // The browser abort cancels the mutation ONLY while it is still undelivered —
  // an already-delivered dispatch completes CLI-side (its retry safety is the
  // CLI DB), never a store-and-forward (plan §3, finding 2).
  const reply = await relayHub().request(
    id,
    "dispatch",
    { session_id: id, command: parsed.body.command, story_id: parsed.body.story_id, fix: parsed.body.fix, client_token: parsed.body.client_token },
    DEADLINE_MS.mutation,
    request.signal,
  );

  return NextResponse.json(reply.body, { status: reply.status });
}
