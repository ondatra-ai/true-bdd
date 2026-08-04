// Builds landing (navigation-only, per the brief's scope: this task ships
// the rail entry + an empty/placeholder landing, nothing more — no file
// editor, no chat mutation target).

import type { ReactNode } from "react";

export default function BuildsPage(): ReactNode {
  return (
    <main data-testid="builds-landing">
      <h1>Builds</h1>
      <p>Build runs will live here in a future task. This section is navigation-only for now.</p>
    </main>
  );
}
