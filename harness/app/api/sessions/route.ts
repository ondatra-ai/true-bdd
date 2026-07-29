import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { browserReadAllowed, forbidden } from "@/app/lib/origin";
import { listSessions } from "@/app/lib/store";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function GET(request: Request) {
  if (!browserReadAllowed(request)) {
    return forbidden();
  }

  return NextResponse.json({ sessions: listSessions(db()) });
}
