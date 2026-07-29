import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { agentAllowed, forbidden } from "@/app/lib/origin";
import { readJsonBody } from "@/app/lib/request-json";
import { inventoryRequestLimitBytes, registerSession } from "@/app/lib/store";

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

  if (
    typeof body.incarnation_id !== "string" ||
    typeof body.folder !== "string" ||
    typeof body.canonical_folder !== "string"
  ) {
    return NextResponse.json({ error: "invalid registration" }, { status: 400 });
  }

  const result = registerSession(db(), {
    incarnation_id: body.incarnation_id,
    folder: body.folder,
    canonical_folder: body.canonical_folder,
    pid: Number(body.pid),
    version: typeof body.version === "string" ? body.version : "",
  });

  // Advertise the server's inventory byte budget so the remote fits its
  // snapshot to it and re-scans smaller on a 413 (plan §1a/§r3-1).
  return NextResponse.json({ ...result, inventory_limit_bytes: inventoryRequestLimitBytes() });
}
