import { NextResponse } from "next/server";

import { browserMutationAllowed, forbidden } from "@/app/lib/origin";
import { relayHub, DEADLINE_MS } from "@/app/lib/relay/hub";
import { readJsonBody } from "@/app/lib/request-json";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

/**
 * POST /api/sessions/:sid/runs/:rid/answer (plan §3) — a relayed answer
 * mutation. The CLI store decides accepted (200) / conflict (409 —
 * conflicting, terminal-race, or cross-owner) / invalid (400) / not_found
 * (404 session_gone or run_pruned); the relay carries the status through.
 */
export async function POST(request: Request, { params }: { params: Promise<{ id: string; rid: string }> }) {
  if (!browserMutationAllowed(request)) {
    return forbidden();
  }

  const { id, rid } = await params;

  const parsed = await readJsonBody(request);
  if (!parsed.ok) {
    return parsed.response;
  }

  if (!relayHub().hasSession(id)) {
    return NextResponse.json({ error: "session_gone" }, { status: 404 });
  }

  const reply = await relayHub().request(
    id,
    "answer",
    { session_id: id, run_id: rid, prompt_id: parsed.body.prompt_id, value: parsed.body.value },
    DEADLINE_MS.mutation,
    request.signal,
  );

  return NextResponse.json(reply.body, { status: reply.status });
}
