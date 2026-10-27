import { createHash } from 'node:crypto';
import { defineConfig } from '@playwright/test';

// A port and a compose project of this copy of the project, derived from its
// own path: two copies running at once (parallel BDD runs) never reuse each
// other's server or tear down each other's stack. teardown.sh derives the
// same project name from the same path.
const digest = createHash('sha256').update(process.cwd()).digest();
const port = 20000 + (digest.readUInt16BE(0) % 20000);
const project = `bdd-${digest.toString('hex').slice(0, 12)}`;

export default defineConfig({
  testDir: './integration',
  timeout: 30_000,
  fullyParallel: false,
  workers: 1,
  reporter: 'json',
  use: {
    baseURL: `http://127.0.0.1:${port}`,
  },
  webServer: {
    command: 'docker compose up --build frontend',
    cwd: '..',
    url: `http://127.0.0.1:${port}`,
    env: { FRONTEND_PORT: String(port), COMPOSE_PROJECT_NAME: project },
    timeout: 300_000,
    reuseExistingServer: true,
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
});
