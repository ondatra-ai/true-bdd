import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    // Playwright specs live under tests/e2e and must never be picked
    // up by vitest; unit/integration suites live under tests/unit.
    include: ["tests/unit/**/*.test.ts"],
  },
});
