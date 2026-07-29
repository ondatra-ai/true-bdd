import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { browserMutationAllowed, forbidden } from "@/app/lib/origin";
import { readJsonBody } from "@/app/lib/request-json";
import { markAbandoned } from "@/app/lib/store";

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
  const result = markAbandoned(db(), id);

  switch (result) {
    case "abandoned":
      return NextResponse.json({ status: "abandoned" }, { status: 200 });
    case "conflict":
      return NextResponse.json({ error: "run not eligible for abandonment" }, { status: 409 });
    case "not_found":
      return NextResponse.json({ error: "unknown run" }, { status: 404 });
  }
}
