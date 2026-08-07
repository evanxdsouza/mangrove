import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  timeout: 60_000,
  fullyParallel: false, // tests share one Mangrove instance and its SQLite state
  workers: 1,
  reporter: [["list"]],
  use: {
    baseURL: process.env.MANGROVE_BASE_URL ?? "http://127.0.0.1:7777",
    trace: "retain-on-failure",
  },
});
