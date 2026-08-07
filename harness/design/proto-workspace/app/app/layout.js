// Inline, self-contained favicon (B&W "T", matching the mockup's square/invert
// language) so every page stops 404-ing on /favicon.ico. Data-URI SVG keeps it
// asset-free — no binary file to track.
const FAVICON =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='4' fill='%23040406'/%3E%3Cpath d='M9 11h14M16 11v12' stroke='%23ffffff' stroke-width='2.6' stroke-linecap='square'/%3E%3C/svg%3E";

export const metadata = {
  title: "TrueBDD Workspace (Next.js prototype)",
  icons: { icon: FAVICON },
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <head>
        <link rel="stylesheet" href="/vendor/tokens.css" />
        <link rel="stylesheet" href="/vendor/workspace.css" />
        <link rel="stylesheet" href="/vendor/mockups.css" />
        <link rel="stylesheet" href="/proto-extra.css" />
      </head>
      <body>{children}</body>
    </html>
  );
}
