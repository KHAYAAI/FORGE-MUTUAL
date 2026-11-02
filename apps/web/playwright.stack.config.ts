import { defineConfig, devices } from "@playwright/test";

// The journeys against the real local stack. Start the stack first (deploy/compose/up.sh); this config does not start
// it, so a failure here is a failure of the real services. Nothing is mocked. The app is served from a production
// build on its own port, with the development sign-in, and its API route forwards to the stack's API.
const STACK = 3110;
const API = "http://127.0.0.1:8300";

export default defineConfig({
  testDir: "./tests/stack",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: [["list"]],
  timeout: 150_000,
  use: {
    baseURL: `http://127.0.0.1:${STACK}`,
    trace: "retain-on-failure",
    launchOptions: process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {},
  },
  projects: [{ name: "stack", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: `npx next start -p ${STACK}`,
    url: `http://127.0.0.1:${STACK}/sign-in`,
    reuseExistingServer: false,
    timeout: 120_000,
    env: { FORGE_AUTH_MODE: "dev", WORKSPACE_API_URL: API },
  },
});
