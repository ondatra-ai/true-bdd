import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { browserReadAllowed, forbidden } from "@/app/lib/origin";
import { runDetail } from "@/app/lib/store";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }) {
  if (!browserReadAllowed(request)) {
    return forbidden();
  }

  const { id } = await params;
  const afterSeqParam = new URL(request.url).searchParams.get("after_seq");
  const afterSeq = afterSeqParam === null ? 0 : Number.parseInt(afterSeqParam, 10) || 0;

  const detail = runDetail(db(), id, afterSeq);
  if (detail === undefined) {
    return NextResponse.json({ error: "unknown run" }, { status: 404 });
  }

  return NextResponse.json(detail);
}
