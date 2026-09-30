import { defineConfig } from "@playwright/test";
import path from "node:path";
const bundle = process.env.TRIP_PACKAGE_DIR;
export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30000,
  reporter: "list",
  use: {
    baseURL: "http://127.0.0.1:18081",
    browserName: "chromium",
    channel: process.env.PLAYWRIGHT_CHANNEL || undefined,
    viewport: { width: 1440, height: 1000 },
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: {
    command: bundle
      ? `"${path.join(bundle, "server.exe")}" -demo -listen 127.0.0.1:18081 -db test-results/e2e-${process.pid}.db -web-dir "${path.join(bundle, "web/dist")}"`
      : `go run ../cmd/server -demo -listen 127.0.0.1:18081 -db test-results/e2e-${process.pid}.db -web-dir dist`,
    url: "http://127.0.0.1:18081",
    reuseExistingServer: false,
    timeout: 120000,
  },
});
