import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { agentAllowed, forbidden } from "@/app/lib/origin";
import { readJsonBody } from "@/app/lib/request-json";
import { appendEvents, type RunEvent } from "@/app/lib/store";

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
  const runId = String(body.run_id ?? "");
  const events = Array.isArray(body.events) ? (body.events as RunEvent[]) : [];
  const answersConsumed = typeof body.answers_consumed === "number" ? body.answers_consumed : 0;

  const result = appendEvents(db(), runId, events, answersConsumed);

  return NextResponse.json(result);
}
