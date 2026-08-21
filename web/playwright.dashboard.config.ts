import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e/dashboard",
  workers: 1,
  fullyParallel: false,
  timeout: 30_000,
  expect: { timeout: 8_000 },
  reporter: "line",
  use: { baseURL: "http://127.0.0.1:4175", channel: "chrome", headless: true },
  webServer: {
    command: "pnpm exec vite --host 127.0.0.1 --port 4175",
    url: "http://127.0.0.1:4175",
    reuseExistingServer: false,
    timeout: 30_000,
  },
});
