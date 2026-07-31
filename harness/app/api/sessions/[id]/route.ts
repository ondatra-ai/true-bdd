import { NextResponse } from "next/server";

import { browserReadAllowed, forbidden } from "@/app/lib/origin";
import { relayHub, DEADLINE_MS } from "@/app/lib/relay/hub";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

/**
 * GET /api/sessions/:id (plan §3). `?view=status` runs ONE `session_status`
 * work item (active run, project history, active owners); the default runs ONE
 * `session_detail` work item (status + inventory in one consistent CLI read).
 * Status mapping (critique §4): 404 session_gone (absent/expired), 504
 * cli_timeout (connected but silent), 502 invalid_cli_reply.
 */
export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }) {
  if (!browserReadAllowed(request)) {
    return forbidden();
  }

  const { id } = await params;

  if (!relayHub().hasSession(id)) {
    return NextResponse.json({ error: "session_gone" }, { status: 404 });
  }

  const status = new URL(request.url).searchParams.get("view") === "status";
  const view = status ? "session_status" : "session_detail";
  const deadline = status ? DEADLINE_MS.status : DEADLINE_MS.inventory;

  // Thread the browser's abort signal so a cancelled read never lingers as
  // undelivered work in the relay (plan §3, finding 2).
  const reply = await relayHub().request(id, "query", { view, session_id: id }, deadline, request.signal);

  return NextResponse.json(reply.body, { status: reply.status });
}
