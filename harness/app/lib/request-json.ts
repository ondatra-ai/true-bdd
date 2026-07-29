/**
 * Shared JSON-body guard for the mutating route families (plan §3.8 /
 * finding 3). A mutating route must require an `application/json`
 * content-type and a parseable JSON object BEFORE it touches any state, so
 * a cross-origin `text/plain` "simple request" carrying a JSON string
 * (which the browser sends WITHOUT a preflight) can never reach the store.
 * The origin/host 403 gate still runs first; this is the second line.
 */

import { NextResponse } from "next/server";

export type JsonBodyResult =
  | { ok: true; body: Record<string, unknown> }
  | { ok: false; response: NextResponse };

/** Whether the request declares an `application/json` content-type. */
function isJsonContentType(request: Request): boolean {
  const header = request.headers.get("content-type");
  if (!header) {
    return false;
  }

  return header.split(";")[0].trim().toLowerCase() === "application/json";
}

/**
 * Requires `application/json` and parses a JSON OBJECT body. Returns a
 * discriminated result so route handlers stay thin: on failure they return
 * the supplied response (415 wrong content-type, 400 unparseable/non-object).
 */
export async function readJsonBody(request: Request): Promise<JsonBodyResult> {
  if (!isJsonContentType(request)) {
    return { ok: false, response: NextResponse.json({ error: "expected application/json" }, { status: 415 }) };
  }

  let parsed: unknown;
  try {
    parsed = await request.json();
  } catch {
    return { ok: false, response: NextResponse.json({ error: "invalid JSON body" }, { status: 400 }) };
  }

  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    return { ok: false, response: NextResponse.json({ error: "expected a JSON object" }, { status: 400 }) };
  }

  return { ok: true, body: parsed as Record<string, unknown> };
}
