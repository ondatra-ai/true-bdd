import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { browserReadAllowed, forbidden } from "@/app/lib/origin";
import { sessionDetail, sessionStatus } from "@/app/lib/store";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }) {
  if (!browserReadAllowed(request)) {
    return forbidden();
  }

  const { id } = await params;

  // The session page polls the LIGHTWEIGHT status view (no inventory
  // snapshot) at 1 Hz and refetches the snapshot only on a generation
  // change (plan §2). The default view still returns the full detail with
  // generation + snapshot from one consistent read (plan §2a).
  if (new URL(request.url).searchParams.get("view") === "status") {
    const status = sessionStatus(db(), id);
    if (status === undefined) {
      return NextResponse.json({ error: "unknown session" }, { status: 404 });
    }

    return NextResponse.json(status);
  }

  const detail = sessionDetail(db(), id);
  if (detail === undefined) {
    return NextResponse.json({ error: "unknown session" }, { status: 404 });
  }

  return NextResponse.json(detail);
}
