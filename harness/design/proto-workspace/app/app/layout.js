export const metadata = {
  title: "TrueBDD Workspace (Next.js prototype)",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <head>
        <link rel="stylesheet" href="/vendor/tokens.css" />
        <link rel="stylesheet" href="/vendor/mockups.css" />
        <link rel="stylesheet" href="/proto-extra.css" />
      </head>
      <body>{children}</body>
    </html>
  );
}
