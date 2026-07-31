/**
 * Mutating-route JSON guard (plan §3.8 / finding 3): a mutating route must
 * require `application/json` and a parseable JSON OBJECT body BEFORE it
 * touches state, so a cross-origin `text/plain` "simple request" carrying a
 * JSON string can never reach the store.
 */

import { describe, expect, it } from "vitest";

import { readJsonBody } from "../../app/lib/request-json";

function post(body: string, contentType?: string): Request {
  const headers: Record<string, string> = {};
  if (contentType !== undefined) {
    headers["content-type"] = contentType;
  }

  return new Request("http://127.0.0.1:4517/api/x", { method: "POST", headers, body });
}

describe("readJsonBody", () => {
  it("accepts an application/json object body", async () => {
    const result = await readJsonBody(post(JSON.stringify({ a: 1 }), "application/json"));
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.body).toEqual({ a: 1 });
    }
  });

  it("accepts application/json with a charset parameter", async () => {
    const result = await readJsonBody(post("{}", "application/json; charset=utf-8"));
    expect(result.ok).toBe(true);
  });

  it("rejects a text/plain body carrying JSON with 415 (the CSRF vector)", async () => {
    const result = await readJsonBody(post(JSON.stringify({ evil: true }), "text/plain"));
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.response.status).toBe(415);
    }
  });

  it("rejects a missing content-type with 415", async () => {
    const result = await readJsonBody(post("{}"));
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.response.status).toBe(415);
    }
  });

  it("rejects an unparseable JSON body with 400", async () => {
    const result = await readJsonBody(post("{not json", "application/json"));
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.response.status).toBe(400);
    }
  });

  it("rejects a non-object JSON body (array / scalar) with 400", async () => {
    for (const body of ["[1,2,3]", "42", '"hi"', "null"]) {
      const result = await readJsonBody(post(body, "application/json"));
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.response.status).toBe(400);
      }
    }
  });
});
