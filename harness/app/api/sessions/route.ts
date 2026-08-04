import { NextResponse } from "next/server";

import { browserReadAllowed, forbidden } from "@/app/lib/origin";
import { relayHub } from "@/app/lib/relay/hub";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
export const maxDuration = 15;

/**
 * GET /api/sessions — REGISTRY-ONLY. Every listed session is connected by
 * definition: there is no reachability, no active_run_id, no
 * inventory_generation. `folder` is the canonical realpath. The workspace
 * edits ONE host project = one connected CLI agent = one session — this list
 * is how the workspace resolves a session id to navigate into.
 */
export async function GET(request: Request) {
  if (!browserReadAllowed(request)) {
    return forbidden();
  }

  const sessions = (await relayHub().listSessions()).map((meta) => ({
    session_id: meta.session_id,
    folder: meta.canonical_folder,
    pid: meta.pid,
    version: meta.version,
    connected_at: meta.connected_at,
  }));

  return NextResponse.json({ sessions });
}
