/**
 * Route-family Origin + Host policy predicates (plan §3.8, asserted by
 * P6/P6b/p19). Pure functions over the request headers — no `next/server`
 * dependency — so the vitest suite can exercise EVERY origin/host case
 * against them directly (plan §4.6). 403 is decided BEFORE any business
 * logic; the NextResponse wrapper lives in origin.ts.
 *
 * Two modes (plan: connect-cli-to-vercel-harness):
 *
 * - **Loopback** (default; `HARNESS_DEPLOYED` is NOT `"1"`): the existing
 *   discipline, byte-for-byte. Every family requires a loopback Host
 *   (127.0.0.1 / localhost); mutations additionally require an Origin whose
 *   authority equals that Host; agent routes accept only an ABSENT Origin (the
 *   Go remote sends none) — a present Origin (incl. the opaque `"null"`) must
 *   match the loopback Host. A spoofed non-loopback Host is rejected (the p19
 *   spoof guard): deployed mode is NEVER inferred from the request Host
 *   (Codex r1 #12 — Host-inference is spoofable and would defeat loopback CSRF
 *   protection on local dev).
 *
 * - **Deployed** (`HARNESS_DEPLOYED === "1"`): admits a non-loopback Host on
 *   GETs and agent routes (the harness runs behind the Vercel proxy). Browser
 *   mutations still require a CSRF Origin check, but the authority the Origin
 *   must match is the EFFECTIVE public host — derived from trusted Vercel
 *   forwarded headers (`x-forwarded-host` / `x-forwarded-proto`) OR an explicit
 *   `HARNESS_PUBLIC_ORIGIN`, NEVER the raw `Host` (Codex r2 #12: raw-Host
 *   compare breaks behind proxies and on preview/custom/alias domains). This
 *   is a request-integrity guard, NOT access control. Agent routes still
 *   accept only an ABSENT Origin (the Go remote sends none); a present Origin
 *   (incl. `"null"`) is rejected unless it matches the effective host.
 *
 * Only `headers.get(name)` is used, so any object with that shape (a real
 * Request or a test double) works.
 */

export interface HeaderCarrier {
  headers: { get(name: string): string | null };
}

function hostname(host: string | null): string {
  return (host ?? "").split(":")[0];
}

function loopbackHost(request: HeaderCarrier): boolean {
  const name = hostname(request.headers.get("host"));

  return name === "127.0.0.1" || name === "localhost";
}

/** Deployed mode is selected SOLELY by an env signal, never by the Host. */
function deployedMode(): boolean {
  return process.env.HARNESS_DEPLOYED === "1";
}

/**
 * The EFFECTIVE public origin behind the Vercel proxy (deployed mode only).
 * Derived from `HARNESS_PUBLIC_ORIGIN` if set, else from trusted forwarded
 * headers. `null` when no effective origin can be established — meaning no
 * Origin can match the CSRF guard (every deployed-mode mutation fails).
 */
function effectiveOrigin(request: HeaderCarrier): string | null {
  const configured = process.env.HARNESS_PUBLIC_ORIGIN;
  if (configured !== undefined && configured.length > 0) {
    try {
      // Normalize to a `${proto}://${host}` origin string.
      const u = new URL(configured);
      return `${u.protocol}//${u.host}`;
    } catch {
      return null;
    }
  }

  const fwdHost = request.headers.get("x-forwarded-host");
  // Reject comma-lists, whitespace, and any multi-valued forwarded host: an
  // attacker who reaches the function with a client-supplied
  // `x-forwarded-host: a, b` must not get to choose the comparison authority.
  // Vercel's edge overrides this header with a single authoritative value; a
  // list here means a misconfigured proxy and is rejected defensively.
  if (fwdHost === null || fwdHost.length === 0 || fwdHost.includes(",") || fwdHost.trim() !== fwdHost) {
    return null;
  }
  const fwdProto = request.headers.get("x-forwarded-proto") ?? "https";
  // Require an http(s) scheme; any other value (or a comma-list) is rejected.
  if (fwdProto !== "http" && fwdProto !== "https") {
    return null;
  }

  return `${fwdProto}://${fwdHost}`;
}

function originMatchesAuthority(origin: string, authority: string): boolean {
  try {
    return new URL(origin).host === authority;
  } catch {
    return false;
  }
}

/** True iff the request Origin matches the loopback Host authority. */
function originMatchesLoopbackHost(request: HeaderCarrier): boolean {
  const origin = request.headers.get("origin");
  const host = request.headers.get("host");
  if (!origin || origin === "null" || !host) {
    return false;
  }

  return originMatchesAuthority(origin, host);
}

/** True iff the request Origin matches the deployed effective origin. */
function originMatchesEffective(request: HeaderCarrier): boolean {
  const origin = request.headers.get("origin");
  if (!origin || origin === "null") {
    return false;
  }
  const effective = effectiveOrigin(request);
  if (effective === null) {
    return false;
  }

  return origin === effective;
}

/** True when a browser-family GET may proceed (Host + mode check). */
export function browserReadAllowed(request: HeaderCarrier): boolean {
  if (deployedMode()) {
    return true;
  }

  return loopbackHost(request);
}

/** True when a browser-family mutation may proceed (Origin AND Host/mode). */
export function browserMutationAllowed(request: HeaderCarrier): boolean {
  if (deployedMode()) {
    // Raw Host is NOT authoritative in deployed mode (Codex r2 #12): the
    // CSRF guard compares Origin to the EFFECTIVE public host derived from
    // trusted forwarded headers / HARNESS_PUBLIC_ORIGIN.
    return originMatchesEffective(request);
  }

  return loopbackHost(request) && originMatchesLoopbackHost(request);
}

/** True when an agent-family request may proceed. */
export function agentAllowed(request: HeaderCarrier): boolean {
  if (deployedMode()) {
    // Agent routes accept only an ABSENT Origin (the Go remote sends none).
    // A PRESENT Origin (incl. the opaque `"null"`) is rejected unless it
    // matches the effective host (matches the browser-mutation guard).
    const origin = request.headers.get("origin");
    if (origin === null) {
      return true;
    }

    return originMatchesEffective(request);
  }

  if (!loopbackHost(request)) {
    return false;
  }

  const origin = request.headers.get("origin");
  // Only an ABSENT Origin header is trusted. A PRESENT Origin — including
  // the opaque `"null"` — must match the loopback Host authority
  // (`originMatchesLoopbackHost` already rejects the string `"null"`), so a
  // cross-origin `null` can never pass.
  if (origin !== null) {
    return originMatchesLoopbackHost(request);
  }

  return true;
}
