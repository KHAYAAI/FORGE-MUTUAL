import { defineConfig, devices } from "@playwright/test";

const PORT = 3100;

// The journeys run against the production build. The workspace API is mocked in the browser, so these tests need no
// backend. Run `npm run build` first. CHROMIUM_PATH may point at a pre-installed browser.
export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"]],
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    trace: "retain-on-failure",
    launchOptions: process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {},
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: `npx next start -p ${PORT}`,
    url: `http://127.0.0.1:${PORT}/sign-in`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    // Closed port: the proxy's unreachable path is tested for real, without a backend.
    env: { WORKSPACE_API_URL: "http://127.0.0.1:9" },
  },
});
