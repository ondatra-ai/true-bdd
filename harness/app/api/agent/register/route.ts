import { NextResponse } from "next/server";

import { agentAllowed, forbidden } from "@/app/lib/origin";
import { relayHub, RELAY_CONFIG } from "@/app/lib/relay/hub";
import { readJsonBody } from "@/app/lib/request-json";

/**
 * The inventory-fit budget the remote fits its snapshot under. It is
 * DISTINCT from the large streamed reply cap (reply_budget_bytes): a
 * per-server `MAX_INVENTORY_REQUEST_BYTES` can be set tiny to exercise the
 * fit ladder WITHOUT 413-ing every read. Absent ⇒ the reply cap (no
 * inventory degradation).
 */
function inventoryBudgetBytes(): number {
  const raw = process.env.MAX_INVENTORY_REQUEST_BYTES;
  const parsed = raw === undefined ? Number.NaN : Number(raw);
  const configured = Number.isFinite(parsed) && parsed > 0 ? parsed : RELAY_CONFIG.replyBudgetBytes;

  return Math.min(configured, RELAY_CONFIG.replyBudgetBytes);
}

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
export const maxDuration = 15;

/**
 * POST /api/agent/register. The client-owned session_id makes a relay
 * restart re-register into the SAME session; the fresh connection_epoch
 * invalidates any prior one. Returns the negotiated reply budget + capability
 * token.
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

  if (
    typeof body.session_id !== "string" ||
    typeof body.folder !== "string" ||
    typeof body.canonical_folder !== "string"
  ) {
    return NextResponse.json({ error: "invalid registration" }, { status: 400 });
  }

  const result = await relayHub().register({
    session_id: body.session_id,
    folder: body.folder,
    canonical_folder: body.canonical_folder,
    pid: Number(body.pid),
    start_identity: typeof body.start_identity === "string" ? body.start_identity : "",
    version: typeof body.version === "string" ? body.version : "",
    connected_at: Date.now(),
  });

  return NextResponse.json({ ...result, inventory_budget_bytes: inventoryBudgetBytes() });
}
