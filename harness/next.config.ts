import type { NextConfig } from "next";

// Standalone output: the Docker runtime layer copies only .next/standalone +
// .next/static (see ../Dockerfile) — no full node_modules, no `next` CLI.
const nextConfig: NextConfig = {
  output: "standalone",
  eslint: {
    // Linting is a separate `npm run lint` step; never block `next build`.
    ignoreDuringBuilds: true,
  },
  async rewrites() {
    return [
      // `_test` is a Next.js App Router PRIVATE folder prefix (excluded from
      // filesystem routing), so the test-only receipt-audit route's real
      // implementation lives at a normal path (`src/app/api/test-receipts/`) and
      // this rewrite maps the binding public URL onto it.
      { source: "/api/_test/receipts", destination: "/api/test-receipts" },
    ];
  },
};

export default nextConfig;
