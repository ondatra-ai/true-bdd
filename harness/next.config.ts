import type { NextConfig } from "next";

// No native-addon carve-out is needed: the relay holds no database (its
// coordination state lives in Redis via `ioredis`, which is pure JS), and no
// sqlite store ships. The previous `serverExternalPackages: ["better-sqlite3"]`
// entry was a stale artifact of a removed design (plan: connect-cli-to-vercel-
// harness → Implementation → next.config.ts).
const nextConfig: NextConfig = {};

export default nextConfig;
