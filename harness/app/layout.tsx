import type { Metadata } from "next";
import type { ReactNode } from "react";

// Poppins 500 (body) + 700 (headings/labels/buttons), pinned via
// @fontsource/poppins — the two weights the S&F Design System permits.
import "@fontsource/poppins/500.css";
import "@fontsource/poppins/700.css";
import "./globals.css";

export const metadata: Metadata = {
  title: "TrueBDD Harness",
  description: "Web harness for driving TrueBDD host projects",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
