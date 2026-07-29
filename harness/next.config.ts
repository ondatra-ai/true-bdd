import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // better-sqlite3 is a native addon; it must be required at runtime by
  // Node, not bundled by the Next compiler (plan §3.1).
  serverExternalPackages: ["better-sqlite3"],
};

export default nextConfig;
