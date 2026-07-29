import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { browserReadAllowed, forbidden } from "@/app/lib/origin";
import { sessionDetail } from "@/app/lib/store";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }) {
  if (!browserReadAllowed(request)) {
    return forbidden();
  }

  const { id } = await params;
  const detail = sessionDetail(db(), id);
  if (detail === undefined) {
    return NextResponse.json({ error: "unknown session" }, { status: 404 });
  }

  return NextResponse.json(detail);
}
