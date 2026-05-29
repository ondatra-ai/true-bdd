import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './integration',
  timeout: 30_000,
  fullyParallel: false,
  workers: 1,
  reporter: 'json',
  use: {
    baseURL: 'http://127.0.0.1:3000',
  },
  webServer: {
    command: 'docker compose up --build frontend',
    cwd: '..',
    url: 'http://127.0.0.1:3000',
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
