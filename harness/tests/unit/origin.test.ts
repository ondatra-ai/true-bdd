/**
 * Origin + Host route-family policy (plan §3.8, asserted end-to-end by
 * P6; unit-covered here per §4.6). Every case is exercised against the
 * pure predicates in origin-policy.ts via a header double.
 */

import { afterEach, describe, expect, it } from "vitest";

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

// The deployed-mode predicates read these two env keys. Capture their original
// values ONCE at collection time and restore them after EVERY test via a single
// suite-level afterEach — so a value set by withDeployedEnv in one test can
// never leak into the next (Codex r2 #4), without registering hooks from inside
// a test body (unsupported in Vitest).
const DEPLOYED_ENV_KEYS = ["HARNESS_DEPLOYED", "HARNESS_PUBLIC_ORIGIN"] as const;
const ORIGINAL_DEPLOYED_ENV: Record<string, string | undefined> = Object.fromEntries(
  DEPLOYED_ENV_KEYS.map((key) => [key, process.env[key]]),
);

afterEach(() => {
  for (const key of DEPLOYED_ENV_KEYS) {
    const value = ORIGINAL_DEPLOYED_ENV[key];
    if (value === undefined) {
      delete process.env[key];
    } else {
      process.env[key] = value;
    }
  }
});

/**
 * Applies the deployed-mode env for a single test: resets both keys, then sets
 * the caller's `vars`. Every key not supplied is deleted, so a value from a
 * previous test can never perturb this one. Restoration is handled once by the
 * suite-level afterEach above — this helper registers NO hooks.
 */
function withDeployedEnv(vars: Record<string, string | undefined>): void {
  for (const key of DEPLOYED_ENV_KEYS) {
    delete process.env[key];
  }
  for (const [key, value] of Object.entries(vars)) {
    if (value === undefined) {
      delete process.env[key];
    } else {
      process.env[key] = value;
    }
  }
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

// ── Deployed mode (plan: connect-cli-to-vercel-harness; Codex r2 #4) ──
// Selected by `HARNESS_DEPLOYED=1`, NEVER by the request Host. The effective
// public host is derived from trusted forwarded headers or HARNESS_PUBLIC_ORIGIN
// (NOT raw Host). Hardened (Codex r1 #4): comma-list / whitespace forwarded
// hosts and non-http(s) protocols are rejected.

describe("deployed mode selection is ENV-driven, never Host-inferred", () => {
  it("admits non-loopback Host only when HARNESS_DEPLOYED=1", () => {
    withDeployedEnv({ HARNESS_DEPLOYED: undefined });
    // No env → loopback discipline → non-loopback Host is 403 even with a
    // spoofed deployed-style Host (the p19 spoof guard).
    expect(browserReadAllowed(req({ host: "true-bdd-app.vercel.app" }))).toBe(false);

    withDeployedEnv({ HARNESS_DEPLOYED: "1" });
    expect(browserReadAllowed(req({ host: "true-bdd-app.vercel.app" }))).toBe(true);
  });
});

describe("deployed browser mutation: Origin must match the EFFECTIVE host", () => {
  it("Origin matching x-forwarded-host is admitted; raw-Host match is NOT", () => {
    withDeployedEnv({ HARNESS_DEPLOYED: "1" });
    const host = "true-bdd-app.vercel.app";
    // Effective host from forwarded headers; raw Host is loopback. A raw-Host
    // compare would reject this (Codex r2 #12 in the plan).
    expect(
      browserMutationAllowed(
        req({ host: `127.0.0.1:4517`, "x-forwarded-host": host, "x-forwarded-proto": "https", origin: `https://${host}` }),
      ),
    ).toBe(true);
    // Raw-Host compare path (no forwarded headers, HARNESS_PUBLIC_ORIGIN unset)
    // → effective host is null → no Origin can match → 403 even with matching
    // Origin to the spoofed Host.
    expect(
      browserMutationAllowed(req({ host: host, origin: `https://${host}` })),
    ).toBe(false);
  });

  it("rejects foreign / null / missing Origin", () => {
    withDeployedEnv({ HARNESS_DEPLOYED: "1" });
    const host = "true-bdd-app.vercel.app";
    const fwd = { "x-forwarded-host": host, "x-forwarded-proto": "https" };
    expect(browserMutationAllowed(req({ ...fwd, host, origin: FOREIGN_ORIGIN }))).toBe(false);
    expect(browserMutationAllowed(req({ ...fwd, host, origin: "null" }))).toBe(false);
    expect(browserMutationAllowed(req({ ...fwd, host }))).toBe(false);
  });

  it("rejects comma-list / whitespace forwarded hosts and non-http(s) protocols", () => {
    withDeployedEnv({ HARNESS_DEPLOYED: "1" });
    const host = "true-bdd-app.vercel.app";
    // Comma-list → no effective host → 403.
    expect(
      browserMutationAllowed(
        req({ "x-forwarded-host": `${host}, evil.example`, "x-forwarded-proto": "https", origin: `https://${host}` }),
      ),
    ).toBe(false);
    // Leading/trailing whitespace → 403.
    expect(
      browserMutationAllowed(
        req({ "x-forwarded-host": ` ${host} `, "x-forwarded-proto": "https", origin: `https://${host}` }),
      ),
    ).toBe(false);
    // Non-http(s) proto → 403.
    expect(
      browserMutationAllowed(
        req({ "x-forwarded-host": host, "x-forwarded-proto": "file", origin: `https://${host}` }),
      ),
    ).toBe(false);
  });

  it("treats an absent x-forwarded-proto as https (the Vercel default)", () => {
    withDeployedEnv({ HARNESS_DEPLOYED: "1" });
    const host = "true-bdd-app.vercel.app";
    expect(
      browserMutationAllowed(req({ "x-forwarded-host": host, origin: `https://${host}` })),
    ).toBe(true);
  });

  it("HARNESS_PUBLIC_ORIGIN overrides and canonicalizes the effective origin", () => {
    withDeployedEnv({ HARNESS_DEPLOYED: "1", HARNESS_PUBLIC_ORIGIN: "https://true-bdd-app.vercel.app/" });
    // No forwarded headers; HARNESS_PUBLIC_ORIGIN supplies the effective origin.
    expect(
      browserMutationAllowed(req({ host: "anything", origin: "https://true-bdd-app.vercel.app" })),
    ).toBe(true);
    // Foreign Origin still rejected.
    expect(browserMutationAllowed(req({ host: "anything", origin: FOREIGN_ORIGIN }))).toBe(false);
  });

  it("HARNESS_PUBLIC_ORIGIN with a malformed value disables mutations (null effective)", () => {
    withDeployedEnv({ HARNESS_DEPLOYED: "1", HARNESS_PUBLIC_ORIGIN: "not-a-url" });
    expect(
      browserMutationAllowed(req({ host: "anything", origin: "https://true-bdd-app.vercel.app" })),
    ).toBe(false);
  });
});

describe("deployed agent family (absent Origin accepted; present Origin must match effective)", () => {
  it("absent Origin + non-loopback Host is admitted (the Go remote sends none)", () => {
    withDeployedEnv({ HARNESS_DEPLOYED: "1" });
    expect(agentAllowed(req({ host: "true-bdd-app.vercel.app" }))).toBe(true);
  });

  it("present Origin is admitted only when it matches the effective host", () => {
    withDeployedEnv({ HARNESS_DEPLOYED: "1" });
    const host = "true-bdd-app.vercel.app";
    const fwd = { "x-forwarded-host": host, "x-forwarded-proto": "https" };
    expect(agentAllowed(req({ ...fwd, host, origin: `https://${host}` }))).toBe(true);
    expect(agentAllowed(req({ ...fwd, host, origin: FOREIGN_ORIGIN }))).toBe(false);
    expect(agentAllowed(req({ ...fwd, host, origin: "null" }))).toBe(false);
  });
});
