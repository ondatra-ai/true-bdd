import type { NextConfig } from "next";

// No native-addon carve-out is needed: the relay holds no database (its
// coordination state lives in Redis via `ioredis`, which is pure JS), and no
// sqlite store ships. The previous `serverExternalPackages: ["better-sqlite3"]`
// entry was a stale artifact of a removed design (plan: connect-cli-to-vercel-
// harness → Implementation → next.config.ts).
// `output: "standalone"` emits `.next/standalone/server.js` with only the
// traced runtime deps, so the Docker image (harness/Dockerfile) and the
// docker-compose `harness` service can run the app with `node server.js`
// instead of the full `next start` + node_modules. See docker-compose.yml.
const nextConfig: NextConfig = {
  output: "standalone",
};

export default nextConfig;
