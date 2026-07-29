import { NextResponse } from "next/server";

import { db } from "@/app/lib/db";
import { browserMutationAllowed, forbidden } from "@/app/lib/origin";
import { readJsonBody } from "@/app/lib/request-json";
import { submitAnswer } from "@/app/lib/store";

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

  const result = submitAnswer(db(), id, parsed.body.prompt_id, parsed.body.value);

  switch (result) {
    case "accepted":
      return NextResponse.json({ status: "accepted" }, { status: 200 });
    case "conflict":
      return NextResponse.json({ error: "conflicting answer" }, { status: 409 });
    case "invalid":
      return NextResponse.json({ error: "invalid answer" }, { status: 400 });
    case "not_found":
      return NextResponse.json({ error: "unknown run" }, { status: 404 });
  }
}
