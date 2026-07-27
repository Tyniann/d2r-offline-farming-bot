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
    maxWorkers: 4,
    setupFiles: "./src/test/setup.ts",
    include: ["src/**/*.test.{ts,tsx}", "electron/**/*.test.ts"],
  },
});
