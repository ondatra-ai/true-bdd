import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { agentAllowed, forbidden } from "@/app/lib/origin";
import { readJsonBody } from "@/app/lib/request-json";
import { pollSession } from "@/app/lib/store";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function POST(request: Request) {
  if (!agentAllowed(request)) {
    return forbidden();
  }

  const parsed = await readJsonBody(request);
  if (!parsed.ok) {
    return parsed.response;
  }
  const body = parsed.body;
  const sessionId = String(body.session_id ?? "");
  const activeRunId = typeof body.active_run_id === "string" && body.active_run_id !== "" ? body.active_run_id : undefined;
  const inventoryUnavailable =
    typeof body.inventory_unavailable === "string" && body.inventory_unavailable !== ""
      ? body.inventory_unavailable
      : undefined;

  const result = pollSession(db(), sessionId, activeRunId, inventoryUnavailable);
  if (result === undefined) {
    return NextResponse.json({ error: "unknown session" }, { status: 404 });
  }

  return NextResponse.json(result);
}
