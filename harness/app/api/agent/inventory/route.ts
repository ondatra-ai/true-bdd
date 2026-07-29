import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { agentAllowed, forbidden } from "@/app/lib/origin";
import { readJsonBody } from "@/app/lib/request-json";
import { inventoryRequestLimitBytes, upsertInventory } from "@/app/lib/store";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function POST(request: Request) {
  if (!agentAllowed(request)) {
    return forbidden();
  }

  // Reject an over-budget snapshot with a streamed 413 before it is
  // materialized or parsed (plan §1a); the remote re-scans smaller.
  const parsed = await readJsonBody(request, inventoryRequestLimitBytes());
  if (!parsed.ok) {
    return parsed.response;
  }
  const body = parsed.body;

  const sessionId = typeof body.session_id === "string" ? body.session_id : "";
  const folder = typeof body.canonical_folder === "string" ? body.canonical_folder : "";
  const token = typeof body.client_token === "string" ? body.client_token : "";
  if (sessionId === "" || folder === "" || token === "" || typeof body.scan_epoch !== "number") {
    return NextResponse.json({ error: "invalid inventory upload" }, { status: 400 });
  }

  const result = upsertInventory(db(), {
    session_id: sessionId,
    canonical_folder: folder,
    scan_epoch: body.scan_epoch,
    snapshot: body.snapshot ?? {},
    client_token: token,
  });

  if ("rejected" in result) {
    return NextResponse.json({ error: result.rejected }, { status: 409 });
  }

  return NextResponse.json(result);
}
