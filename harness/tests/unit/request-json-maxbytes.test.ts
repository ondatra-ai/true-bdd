/**
 * readJsonBody streamed byte-limit table (plan §1a/§4.6). A mutating route
 * that accepts an inventory upload must reject an over-budget body with a
 * 413 BEFORE JSON parsing — route-specific (`MAX_INVENTORY_REQUEST_BYTES`,
 * default 4 MiB), so a pathological snapshot can never be materialized in
 * memory just to be rejected. The limit is enforced on BYTES, so a
 * multibyte payload is measured by its encoded length, not its char count.
 *
 * The target signature `readJsonBody(request, maxBytes)` does NOT exist yet
 * (the current export takes one argument and ignores any limit): the
 * function is accessed through a pinned cast so the file typechecks today
 * and the 413 tables are RED at runtime until the limit is implemented.
 */

import { describe, expect, it } from "vitest";

import { readJsonBody, type JsonBodyResult } from "../../app/lib/request-json";

type ReadJsonBodyWithMax = (request: Request, maxBytes?: number) => Promise<JsonBodyResult>;

const readJsonBodyMax = readJsonBody as unknown as ReadJsonBodyWithMax;

/** A JSON request whose serialized body is exactly `bytes` bytes (ASCII). */
function jsonOfBytes(bytes: number): { request: Request; body: string } {
  // {"a":"<pad>"} — the fixed frame is 8 bytes, pad fills the rest.
  const frame = 8;
  if (bytes < frame) {
    throw new Error(`cannot build a JSON object body under ${frame} bytes`);
  }
  const body = `{"a":"${"x".repeat(bytes - frame)}"}`;
  if (Buffer.byteLength(body, "utf8") !== bytes) {
    throw new Error(`frame math wrong: got ${Buffer.byteLength(body, "utf8")}, want ${bytes}`);
  }

  return {
    body,
    request: new Request("http://127.0.0.1:4517/api/agent/inventory", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body,
    }),
  };
}

describe("readJsonBody(request, maxBytes)", () => {
  it("accepts a body of exactly maxBytes (cap) and maxBytes-1 (cap-1)", async () => {
    const cap = 64;
    const atCap = await readJsonBodyMax(jsonOfBytes(cap).request, cap);
    expect(atCap.ok).toBe(true);

    const belowCap = await readJsonBodyMax(jsonOfBytes(cap - 1).request, cap);
    expect(belowCap.ok).toBe(true);
  });

  it("rejects a body of maxBytes+1 (cap+1) with 413 before parsing", async () => {
    const cap = 64;
    const overCap = await readJsonBodyMax(jsonOfBytes(cap + 1).request, cap);
    expect(overCap.ok).toBe(false);
    if (!overCap.ok) {
      expect(overCap.response.status).toBe(413);
    }
  });

  it("measures the limit in BYTES, not characters (multibyte boundary)", async () => {
    // "😀" encodes to 4 UTF-8 bytes. A single-emoji value is 2 chars but the
    // body is well over a tiny byte cap ⇒ 413.
    const body = JSON.stringify({ a: "😀😀😀😀" }); // 16 payload bytes + frame
    const request = new Request("http://127.0.0.1:4517/api/agent/inventory", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body,
    });
    const cap = 12; // fewer bytes than the encoded body
    expect(Buffer.byteLength(body, "utf8")).toBeGreaterThan(cap);

    const result = await readJsonBodyMax(request, cap);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.response.status).toBe(413);
    }
  });

  it("a body within a generous cap still parses to the JSON object", async () => {
    const result = await readJsonBodyMax(jsonOfBytes(32).request, 4 * 1024 * 1024);
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(typeof result.body.a).toBe("string");
    }
  });
});
