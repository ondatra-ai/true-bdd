/**
 * Route-family Origin + Host policy predicates (plan §3.8, asserted by
 * P6). Pure functions over the request headers — no `next/server`
 * dependency — so the vitest suite can exercise EVERY origin/host case
 * against them directly (plan §4.6). 403 is decided BEFORE any business
 * logic; the NextResponse wrapper lives in origin.ts.
 *
 * - Browser mutation routes: require loopback Host AND an Origin whose
 *   authority equals that Host (missing / `null` / foreign Origin → 403).
 * - Agent routes: require loopback Host; only a genuinely ABSENT Origin is
 *   accepted (the Go remote sends none). Any PRESENT Origin — including the
 *   literal string `"null"`, the serialization of an opaque origin — must
 *   match the Host authority; an opaque `null` has no matching authority
 *   and is therefore rejected (finding 3).
 * - GETs: Host check only.
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

function originMatchesHost(request: HeaderCarrier): boolean {
  const origin = request.headers.get("origin");
  const host = request.headers.get("host");
  if (!origin || origin === "null" || !host) {
    return false;
  }

  try {
    return new URL(origin).host === host;
  } catch {
    return false;
  }
}

/** True when a browser-family GET may proceed (Host check only). */
export function browserReadAllowed(request: HeaderCarrier): boolean {
  return loopbackHost(request);
}

/** True when a browser-family mutation may proceed (Origin AND Host). */
export function browserMutationAllowed(request: HeaderCarrier): boolean {
  return loopbackHost(request) && originMatchesHost(request);
}

/** True when an agent-family request may proceed. */
export function agentAllowed(request: HeaderCarrier): boolean {
  if (!loopbackHost(request)) {
    return false;
  }

  const origin = request.headers.get("origin");
  // Only an ABSENT Origin header is trusted. A PRESENT Origin — including
  // the opaque `"null"` — must match the loopback Host authority
  // (`originMatchesHost` already rejects the string `"null"`), so a
  // cross-origin `null` can never pass.
  if (origin !== null) {
    return originMatchesHost(request);
  }

  return true;
}
