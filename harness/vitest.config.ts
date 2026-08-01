import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    // Playwright specs live in the separate self-contained suite at the
    // repo-root tests/harness/ package and must never be picked up by
    // vitest; unit/integration suites live under tests/unit.
    include: ["tests/unit/**/*.test.ts"],
  },
});
