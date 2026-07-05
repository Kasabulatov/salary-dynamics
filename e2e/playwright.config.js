// E2E config. Tests run against the docker-compose stack:
//   frontend http://localhost:5173, backend http://localhost:8080
// Fixtures are USD-only so no external rate API is ever needed — the suite
// must never flake on third-party availability.
import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  timeout: 60_000,
  retries: 1,
  workers: 1, // shared database — keep runs serial
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:5173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
})
