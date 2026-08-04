// Sessions-list placeholder (startup scaffolding — test-author, behavior-free).
//
// Returns a 200 so the harness container passes e2e readiness (GET /). It is a
// PLACEHOLDER only: the real sessions list — one row per connected CLI agent,
// resolved from `GET /api/sessions` — is coder work. No data fetching, no
// session resolution, no testids the specs depend on.

import type { ReactNode } from "react";

export default function SessionsPlaceholderPage(): ReactNode {
  return (
    <main>
      <h1>TrueBDD harness</h1>
      <p>Sessions list is not implemented yet.</p>
    </main>
  );
}
