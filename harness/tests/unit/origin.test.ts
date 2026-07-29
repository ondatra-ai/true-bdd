/**
 * Origin + Host route-family policy (plan §3.8, asserted end-to-end by
 * P6; unit-covered here per §4.6). Every case is exercised against the
 * pure predicates in origin-policy.ts via a header double.
 */

import { describe, expect, it } from "vitest";

import { agentAllowed, browserMutationAllowed, browserReadAllowed, type HeaderCarrier } from "../../app/lib/origin-policy";

const LOOPBACK = "127.0.0.1:4517";
const GOOD_ORIGIN = "http://127.0.0.1:4517";
const FOREIGN_ORIGIN = "http://evil.example";
const WRONG_HOST = "harness.example.com";

function req(headers: Record<string, string | undefined>): HeaderCarrier {
  return {
    headers: {
      get: (name: string) => headers[name.toLowerCase()] ?? null,
    },
  };
}

describe("browser mutation family (Origin AND Host)", () => {
  it("allows only the exact loopback Origin matching the loopback Host", () => {
    expect(browserMutationAllowed(req({ host: LOOPBACK, origin: GOOD_ORIGIN }))).toBe(true);
    expect(browserMutationAllowed(req({ host: "localhost:4517", origin: "http://localhost:4517" }))).toBe(true);
  });

  it("rejects foreign / missing / null Origin", () => {
    expect(browserMutationAllowed(req({ host: LOOPBACK, origin: FOREIGN_ORIGIN }))).toBe(false);
    expect(browserMutationAllowed(req({ host: LOOPBACK }))).toBe(false);
    expect(browserMutationAllowed(req({ host: LOOPBACK, origin: "null" }))).toBe(false);
  });

  it("rejects a correct Origin against a wrong or missing Host", () => {
    expect(browserMutationAllowed(req({ host: WRONG_HOST, origin: GOOD_ORIGIN }))).toBe(false);
    expect(browserMutationAllowed(req({ origin: GOOD_ORIGIN }))).toBe(false);
    // Origin authority must match the Host authority, not just be loopback.
    expect(browserMutationAllowed(req({ host: LOOPBACK, origin: "http://127.0.0.1:9999" }))).toBe(false);
  });
});

describe("agent family (loopback Host; foreign Origin rejected; absent Origin allowed)", () => {
  it("allows an absent Origin on a loopback Host", () => {
    expect(agentAllowed(req({ host: LOOPBACK }))).toBe(true);
    expect(agentAllowed(req({ host: "localhost:4517" }))).toBe(true);
  });

  it("allows a matching Origin and rejects a present foreign Origin", () => {
    expect(agentAllowed(req({ host: LOOPBACK, origin: GOOD_ORIGIN }))).toBe(true);
    expect(agentAllowed(req({ host: LOOPBACK, origin: FOREIGN_ORIGIN }))).toBe(false);
  });

  it("rejects the literal opaque Origin `null` even on a loopback Host (finding 3)", () => {
    // Inverted review repro: `Origin: null` is a PRESENT opaque origin with
    // no matching loopback authority — only a genuinely ABSENT header is
    // trusted, so a cross-origin `text/plain` fetch cannot pass as an agent.
    expect(agentAllowed(req({ host: LOOPBACK, origin: "null" }))).toBe(false);
    expect(agentAllowed(req({ host: "localhost:4517", origin: "null" }))).toBe(false);
  });

  it("rejects any request on a wrong or missing Host", () => {
    expect(agentAllowed(req({ host: WRONG_HOST }))).toBe(false);
    expect(agentAllowed(req({ host: WRONG_HOST, origin: GOOD_ORIGIN }))).toBe(false);
    expect(agentAllowed(req({}))).toBe(false);
  });
});

describe("GET family (Host check only)", () => {
  it("allows a loopback Host and rejects everything else", () => {
    expect(browserReadAllowed(req({ host: LOOPBACK }))).toBe(true);
    expect(browserReadAllowed(req({ host: "localhost" }))).toBe(true);
    // A GET ignores Origin entirely.
    expect(browserReadAllowed(req({ host: LOOPBACK, origin: FOREIGN_ORIGIN }))).toBe(true);
    expect(browserReadAllowed(req({ host: WRONG_HOST }))).toBe(false);
    expect(browserReadAllowed(req({}))).toBe(false);
  });
});
