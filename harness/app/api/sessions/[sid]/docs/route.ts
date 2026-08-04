import { NextResponse } from "next/server";

import { browserReadAllowed, forbidden } from "@/app/lib/origin";
import { DEADLINE_MS, relayHub } from "@/app/lib/relay/hub";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
export const maxDuration = 15;

/**
 * GET /api/sessions/:sid/docs — the fixed-schema document manifest (plan
 * Slice 0): turns into a `doc_tree` work item the relay carries to the CLI.
 * The relay never touches the filesystem; it only relays the CLI's reply.
 */
export async function GET(request: Request, { params }: { params: Promise<{ sid: string }> }) {
  if (!browserReadAllowed(request)) {
    return forbidden();
  }

  const { sid } = await params;
  const reply = await relayHub().request(sid, "doc_tree", {}, DEADLINE_MS.status);

  return NextResponse.json(reply.body, { status: reply.status });
}
