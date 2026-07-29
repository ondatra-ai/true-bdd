import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { browserMutationAllowed, forbidden } from "@/app/lib/origin";
import { readJsonBody } from "@/app/lib/request-json";
import { dispatchRun } from "@/app/lib/store";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  if (!browserMutationAllowed(request)) {
    return forbidden();
  }

  const { id } = await params;

  const parsed = await readJsonBody(request);
  if (!parsed.ok) {
    return parsed.response;
  }

  const result = dispatchRun(db(), id, parsed.body);

  switch (result.kind) {
    case "created":
      return NextResponse.json({ run_id: result.run_id }, { status: 201 });
    case "deduped":
      return NextResponse.json({ run_id: result.run_id }, { status: 200 });
    case "invalid":
      return NextResponse.json({ error: "invalid command or body" }, { status: 400 });
    case "not_found":
      return NextResponse.json({ error: "unknown session" }, { status: 404 });
    case "conflict":
      return NextResponse.json({ error: "session unreachable or has a non-terminal run" }, { status: 409 });
  }
}
