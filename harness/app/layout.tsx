// Root HTML layout shell. Carries the design-system token mirror
// (globals.css → harness/design/system/tokens.css, P7) but no workspace
// logic itself — the workspace shell (rail/sidebar/file view/chat) lives in
// the persistent `(workspace)` route-group layout.

import type { ReactNode } from "react";

import "./globals.css";

export const metadata = {
  title: "TrueBDD harness",
};

export default function RootLayout({ children }: { children: ReactNode }): ReactNode {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
