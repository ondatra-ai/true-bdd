import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { browserMutationAllowed, forbidden } from "@/app/lib/origin";
import { readJsonBody } from "@/app/lib/request-json";
import { requestRefresh } from "@/app/lib/store";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  if (!browserMutationAllowed(request)) {
    return forbidden();
  }

  const parsed = await readJsonBody(request);
  if (!parsed.ok) {
    return parsed.response;
  }

  const { id } = await params;
  if (!requestRefresh(db(), id)) {
    return NextResponse.json({ error: "unknown session" }, { status: 404 });
  }

  return new NextResponse(null, { status: 202 });
}
