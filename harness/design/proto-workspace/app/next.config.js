/**
 * Golden captures build the prototype in PRODUCTION mode into an isolated
 * dist dir (PROTO_DIST_DIR, set by tests/harness/helpers/proto-baseline.ts)
 * so a long-running `next dev` on the default `.next` is never disturbed —
 * and so the committed design goldens are rendered by the SAME `next build`
 * pipeline as the production harness app (dev-mode rendering differs at the
 * sub-pixel level and contaminates the w17 pixel-parity gate).
 */
const nextConfig = {
  distDir: process.env.PROTO_DIST_DIR || ".next",
  // The prototype is a design artifact, not production code — its build-time
  // lint findings (unescaped quotes etc.) are irrelevant to the golden render
  // and must not block a capture.
  eslint: { ignoreDuringBuilds: true },
};

module.exports = nextConfig;
