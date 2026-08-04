import { NextResponse } from "next/server";

import { browserReadAllowed, forbidden } from "@/app/lib/origin";
import { DEADLINE_MS, relayHub } from "@/app/lib/relay/hub";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
export const maxDuration = 15;

/**
 * GET /api/sessions/:sid/docs/read?path= — a document's current bytes,
 * existence/parse status, and content-derived revision (plan Slice 0): turns
 * into a `doc_read` work item the relay carries to the CLI.
 */
export async function GET(request: Request, { params }: { params: Promise<{ sid: string }> }) {
  if (!browserReadAllowed(request)) {
    return forbidden();
  }

  const { sid } = await params;
  const url = new URL(request.url);
  const path = url.searchParams.get("path") ?? "";

  const reply = await relayHub().request(sid, "doc_read", { path }, DEADLINE_MS.status);

  return NextResponse.json(reply.body, { status: reply.status });
}
