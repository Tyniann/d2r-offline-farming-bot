import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./electron/e2e",
  workers: 1,
  fullyParallel: false,
  timeout: 20_000,
  expect: { timeout: 8_000 },
  reporter: "line",
});
