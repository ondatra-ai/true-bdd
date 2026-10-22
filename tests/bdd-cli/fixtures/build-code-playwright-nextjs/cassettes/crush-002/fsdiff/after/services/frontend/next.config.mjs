/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // ESLint is not a dependency of this service; the architecture quality gate runs
  // type checking separately via services/frontend/tsconfig.json. Keep `next build`
  // from failing the container build on lint tooling that is intentionally absent.
  eslint: { ignoreDuringBuilds: true },
};

export default nextConfig;
