import { NextResponse } from "next/server";

import { browserMutationAllowed, forbidden } from "@/app/lib/origin";
import { DEADLINE_MS, relayHub } from "@/app/lib/relay/hub";
import { readJsonBody } from "@/app/lib/request-json";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
// A real Claude turn (a10) can legitimately run several minutes; on Vercel
// this route would need an async/polling redesign to survive a serverless
// function's hard ceiling, but locally (next start, the e2e target) there is
// no such ceiling — this just documents the intent for a future deploy.
export const maxDuration = 800;

/**
 * POST /api/sessions/:sid/chat {conversation, current_path, current_content}
 * — the docked-chat surface (plan Slice 5): a narrowly-scoped `chat` work
 * item the relay carries to the CLI (a deterministic driver or a real Claude
 * turn), replying with `{reply_text, edit: {path, new_content} | null}`. This
 * is a MUTATION-adjacent route (it can, via the browser applying `edit`,
 * lead to a doc_write) so it is Origin+Host gated like doc_write.
 */
export async function POST(request: Request, { params }: { params: Promise<{ sid: string }> }) {
  if (!browserMutationAllowed(request)) {
    return forbidden();
  }

  const parsed = await readJsonBody(request);
  if (!parsed.ok) {
    return parsed.response;
  }
  const body = parsed.body;

  if (!Array.isArray(body.conversation)) {
    return NextResponse.json({ error: "invalid" }, { status: 400 });
  }

  const { sid } = await params;
  const reply = await relayHub().request(
    sid,
    "chat",
    {
      conversation: body.conversation,
      current_path: typeof body.current_path === "string" ? body.current_path : null,
      current_content: typeof body.current_content === "string" ? body.current_content : null,
    },
    DEADLINE_MS.chat,
  );

  return NextResponse.json(reply.body, { status: reply.status });
}
