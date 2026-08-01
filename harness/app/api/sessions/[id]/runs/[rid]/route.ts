import { NextResponse } from "next/server";

import { browserReadAllowed, forbidden } from "@/app/lib/origin";
import { relayHub, DEADLINE_MS } from "@/app/lib/relay/hub";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
// Vercel function budget: above the runDetail 5s wait + slack (plan r1 #14).
export const maxDuration = 15;

/**
 * GET /api/sessions/:sid/runs/:rid (plan §3) — ONE `run_detail` work item.
 * Returns the ENTIRE current capped window (no naive paging; `?after_seq` is
 * reserved). Project-wide reads are allowed through any live session; the
 * cross-owner `answerable` flag is set by the CLI (plan §1.1).
 */
export async function GET(request: Request, { params }: { params: Promise<{ id: string; rid: string }> }) {
  if (!browserReadAllowed(request)) {
    return forbidden();
  }

  const { id, rid } = await params;

  if (!(await relayHub().hasSession(id))) {
    return NextResponse.json({ error: "session_gone" }, { status: 404 });
  }

  const reply = await relayHub().request(
    id,
    "query",
    { view: "run_detail", session_id: id, run_id: rid },
    DEADLINE_MS.runDetail,
    request.signal,
  );

  return NextResponse.json(reply.body, { status: reply.status });
}
