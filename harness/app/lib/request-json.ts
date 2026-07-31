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
 * the supplied response (415 wrong content-type, 413 over-budget body, 400
 * unparseable/non-object).
 *
 * When `maxBytes` is supplied the body is read under a STREAMED byte limit
 * (plan §1a): a pathological inventory upload is rejected with 413 BEFORE it
 * is materialized in memory or JSON-parsed. The limit is enforced on ENCODED
 * BYTES, so a multibyte payload is measured by its UTF-8 length, not its
 * character count.
 */
export async function readJsonBody(request: Request, maxBytes?: number): Promise<JsonBodyResult> {
  if (!isJsonContentType(request)) {
    return { ok: false, response: NextResponse.json({ error: "expected application/json" }, { status: 415 }) };
  }

  let raw: string;
  if (maxBytes === undefined) {
    raw = await request.text();
  } else {
    const limited = await readLimitedText(request, maxBytes);
    if (!limited.ok) {
      return { ok: false, response: NextResponse.json({ error: "request body too large" }, { status: 413 }) };
    }
    raw = limited.text;
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return { ok: false, response: NextResponse.json({ error: "invalid JSON body" }, { status: 400 }) };
  }

  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    return { ok: false, response: NextResponse.json({ error: "expected a JSON object" }, { status: 400 }) };
  }

  return { ok: true, body: parsed as Record<string, unknown> };
}

type LimitedText = { ok: true; text: string } | { ok: false };

/**
 * Reads the request body as UTF-8 text under a hard byte cap, aborting the
 * stream the instant the accumulated encoded length exceeds `maxBytes`. A
 * body of exactly `maxBytes` is accepted; `maxBytes + 1` is rejected.
 */
async function readLimitedText(request: Request, maxBytes: number): Promise<LimitedText> {
  const stream = request.body;
  if (stream === null) {
    const text = await request.text();
    if (new TextEncoder().encode(text).byteLength > maxBytes) {
      return { ok: false };
    }

    return { ok: true, text };
  }

  const reader = stream.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;

  for (;;) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    if (value === undefined) {
      continue;
    }

    total += value.byteLength;
    if (total > maxBytes) {
      await reader.cancel();

      return { ok: false };
    }
    chunks.push(value);
  }

  const merged = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    merged.set(chunk, offset);
    offset += chunk.byteLength;
  }

  return { ok: true, text: new TextDecoder().decode(merged) };
}
