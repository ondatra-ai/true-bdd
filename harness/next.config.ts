import type { NextConfig } from "next";

// Standalone output: the Docker runtime layer copies only .next/standalone +
// .next/static (see ../Dockerfile) — no full node_modules, no `next` CLI.
const nextConfig: NextConfig = {
  output: "standalone",
  eslint: {
    // Linting is a separate `npm run lint` step; never block `next build`.
    ignoreDuringBuilds: true,
  },
};

export default nextConfig;
