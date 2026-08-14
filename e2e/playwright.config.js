// @ts-check
const { defineConfig, devices } = require("@playwright/test");

// Points at an already-running server (Docker container or `make run`) — see e2e/README.md.
// No `webServer` block: the app needs libvips (govips/cgo), which this dev sandbox doesn't have
// installed directly, so the server always runs in the Debian/libvips Docker image, started
// separately before `npm test`.
const BASE_URL = process.env.BASE_URL || "http://localhost:3000";

// Uses the system-installed Chromium (snap, on this sandbox) instead of a Playwright-managed
// browser download — keeps this suite runnable without a `playwright install` step. Any real
// Chromium/Chrome install works; override CHROMIUM_PATH if it's somewhere else.
const CHROMIUM_PATH = process.env.CHROMIUM_PATH || "/snap/bin/chromium";

module.exports = defineConfig({
  testDir: "./tests",
  fullyParallel: false, // tests share one server-side SQLite database and build on each other
  workers: 1,
  retries: 0,
  reporter: [["list"]],
  timeout: 30_000,
  use: {
    baseURL: BASE_URL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        launchOptions: {
          executablePath: CHROMIUM_PATH,
          args: ["--no-sandbox", "--disable-gpu"],
        },
      },
    },
  ],
});
