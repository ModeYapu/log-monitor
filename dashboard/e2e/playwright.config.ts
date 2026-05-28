import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './specs',
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: [['list']],
  timeout: 30000,
  use: {
    baseURL: 'http://127.0.0.1/logmon/',
    ignoreHTTPSErrors: true,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'setup',
      testMatch: /setup\.ts/,
    },
    {
      name: 'tests',
      testDir: './specs',
      dependencies: ['setup'],
      use: {
        storageState: 'e2e/.auth/admin.json',
      },
    },
  ],
});
