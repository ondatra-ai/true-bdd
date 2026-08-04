import { randomUUID } from "node:crypto";

import { NextResponse } from "next/server";

import { browserMutationAllowed, forbidden } from "@/app/lib/origin";
import { DEADLINE_MS, relayHub } from "@/app/lib/relay/hub";
import { readJsonBody } from "@/app/lib/request-json";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
export const maxDuration = 15;

/**
 * POST /api/sessions/:sid/docs/write {path, content, base_revision,
 * client_token} — the S1 persistence backend (plan Slice 0): a revisioned,
 * serialized `doc_write` work item the relay carries to the CLI. `doc_write`
 * is a MUTATION → exact Origin+Host enforced BEFORE business logic. The
 * relay never touches the filesystem; it relays the CLI's write receipt
 * verbatim (`{receipt: {path, committed_revision, content_hash, bytes}}`,
 * `work_id` added below so the browser can correlate with the receipt-audit
 * hook).
 */
export async function POST(request: Request, { params }: { params: Promise<{ sid: string }> }) {
  if (!browserMutationAllowed(request)) {
    return forbidden();
  }

  const parsed = await readJsonBody(request);
  if (!parsed.ok) {
    return parsed.response;
  }
  const body = parsed.body;

  if (
    typeof body.path !== "string" ||
    typeof body.content !== "string" ||
    typeof body.base_revision !== "string"
  ) {
    return NextResponse.json({ error: "invalid" }, { status: 400 });
  }

  const clientToken = typeof body.client_token === "string" && body.client_token.length > 0
    ? body.client_token
    : randomUUID();

  const { sid } = await params;
  const reply = await relayHub().request(
    sid,
    "doc_write",
    { path: body.path, content: body.content, base_revision: body.base_revision, client_token: clientToken },
    DEADLINE_MS.mutation,
  );

  const responseBody =
    reply.status === 200 && typeof reply.body === "object" && reply.body !== null
      ? { receipt: { ...(reply.body as { receipt?: object }).receipt, work_id: reply.workId } }
      : reply.body;

  return NextResponse.json(responseBody, { status: reply.status });
}
