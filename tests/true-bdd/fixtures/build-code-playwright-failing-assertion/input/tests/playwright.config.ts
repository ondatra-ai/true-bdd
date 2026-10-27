import { createHash } from 'node:crypto';
import { defineConfig } from '@playwright/test';

// A port of this copy of the project, derived from its own path: two copies
// running at once (parallel BDD runs) never share a server or a port.
const port = 20000 + (createHash('sha256').update(process.cwd()).digest().readUInt16BE(0) % 20000);

export default defineConfig({
  testDir: './integration',
  timeout: 30_000,
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: 'json',
  use: {
    baseURL: `http://127.0.0.1:${port}`,
  },
  webServer: {
    command: 'node ../services/web/server.js',
    url: `http://127.0.0.1:${port}/`,
    env: { PORT: String(port) },
    timeout: 30_000,
    reuseExistingServer: false,
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
});
