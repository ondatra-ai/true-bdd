/**
 * Route-family Origin + Host policy (plan §3.8, asserted by P6). The
 * policy predicates live in origin-policy.ts (pure, `next/server`-free,
 * vitest-testable); this module re-exports them for the route handlers
 * and adds the shared 403 NextResponse. 403 is decided BEFORE any
 * business logic.
 */

import { NextResponse } from "next/server";

export { agentAllowed, browserMutationAllowed, browserReadAllowed } from "./origin-policy";

/** The uniform 403 body for a rejected route-family request. */
export function forbidden(): NextResponse {
  return NextResponse.json({ error: "origin/host policy violation" }, { status: 403 });
}
