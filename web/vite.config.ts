import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "./",
  plugins: [react()],
  build: {
    outDir: "../internal/api/ui/dist",
    emptyOutDir: true,
  },
  test: {
    environment: "jsdom",
    pool: "forks",
    maxWorkers: 4,
    testTimeout: 15_000,
    setupFiles: "./src/test/setup.ts",
    include: ["src/**/*.test.{ts,tsx}", "electron/**/*.test.ts"],
  },
});
