import { NextResponse } from "next/server";

import { browserReadAllowed, forbidden } from "@/app/lib/origin";
import { relayHub } from "@/app/lib/relay/hub";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
export const maxDuration = 15;

/**
 * Serves GET /api/_test/receipts?sid= — the test-only relay receipt-audit
 * hook (S1 oracle, plan Slice 0). The PUBLIC path starts with `_test`, which
 * Next.js App Router treats as a private folder excluded from filesystem
 * routing, so `next.config.ts` REWRITES `/api/_test/receipts` to this route
 * (`/api/test-receipts`) — the handler itself lives at a normal, routable
 * path. Records every CLI-originated `doc_write` receipt keyed by `work_id`,
 * readable ONLY when HARNESS_RECEIPT_AUDIT=1 (the test harness sets this; it
 * must be OFF in production — a 404 otherwise so the route surface itself
 * does not leak into a real deployment).
 */
export async function GET(request: Request) {
  if (process.env.HARNESS_RECEIPT_AUDIT !== "1") {
    return NextResponse.json({ error: "not_found" }, { status: 404 });
  }

  if (!browserReadAllowed(request)) {
    return forbidden();
  }

  const url = new URL(request.url);
  const sid = url.searchParams.get("sid") ?? "";

  const receipts = await relayHub().listReceipts(sid);

  return NextResponse.json({ receipts });
}
