// Workspace overview (Home landing, P1). No single backing file — the chat
// converses here but performs no file edit (P10).

import type { ReactNode } from "react";

export default function WorkspaceHomePage(): ReactNode {
  return (
    <main data-testid="home-landing">
      <h1>Workspace overview</h1>
      <p>Pick a section from the rail on the left — Architecture, Product, or Builds.</p>
    </main>
  );
}
