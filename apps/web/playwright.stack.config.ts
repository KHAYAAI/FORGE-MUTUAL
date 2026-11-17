import { readFileSync } from "node:fs";
import { join } from "node:path";
import { defineConfig, devices } from "@playwright/test";

// The journeys against the real local stack. Start the stack first (deploy/compose/up.sh); this config does not start
// it, so a failure here is a failure of the real services. Nothing is mocked. The app is served from a production
// build on its own port, with the development sign-in, and its API route forwards to the stack's API.
const STACK = 3110;

// The stack says how it was started in deploy/compose/.env. With `up.sh mtls` the API serves only callers that present a
// certificate, so the app's server is given its own (svc-web), exactly as it would be in production.
const compose = join(__dirname, "..", "..", "deploy", "compose");
const mutualTLS = /^STACK_TLS=1$/m.test(readFileSync(join(compose, ".env"), "utf8"));
const API = `${mutualTLS ? "https" : "http"}://127.0.0.1:8300`;
const certificate: Record<string, string> = mutualTLS
  ? {
      TLS_CERT_FILE: join(compose, ".pki", "svc-web.crt"),
      TLS_KEY_FILE: join(compose, ".pki", "svc-web.key"),
      TLS_CA_FILE: join(compose, ".pki", "ca.crt"),
    }
  : {};

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
    env: { FORGE_AUTH_MODE: "dev", WORKSPACE_API_URL: API, ...certificate },
  },
});
