import { defineConfig, devices } from "@playwright/test";

// Three ways in, each on its own port, from the same production build:
//   3200  the dev sign-in (FORGE_AUTH_MODE=dev), used by the journeys in tests/*.spec.ts
//   3201  the OIDC sign-in (FORGE_AUTH_MODE=oidc), used by tests/oidc/*.spec.ts, against the development identity provider
//   3300  the development identity provider (tools/devidp)
//   3400  a fake backoffice-api that records what the proxy sends
const DEV = 3200;
const OIDC = 3201;
const IDP = 3300;
const UPSTREAM = 3400;

export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"]],
  use: {
    trace: "retain-on-failure",
    launchOptions: process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {},
  },
  projects: [
    {
      name: "dev",
      testIgnore: [/oidc\//, /stack\//],
      use: { ...devices["Desktop Chrome"], baseURL: `http://127.0.0.1:${DEV}` },
    },
    {
      name: "oidc",
      testMatch: /oidc\/.*\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], baseURL: `http://127.0.0.1:${OIDC}` },
    },
  ],
  webServer: [
    {
      command: `npx next start -p ${DEV}`,
      url: `http://127.0.0.1:${DEV}/sign-in`,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      env: { FORGE_AUTH_MODE: "dev", BACKOFFICE_API_URL: "http://127.0.0.1:9" },
    },
    {
      command: `npx next start -p ${OIDC}`,
      url: `http://127.0.0.1:${OIDC}/sign-in`,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      env: {
        FORGE_AUTH_MODE: "oidc",
        FORGE_PUBLIC_URL: `http://127.0.0.1:${OIDC}`,
        OIDC_ISSUER: `http://127.0.0.1:${IDP}`,
        OIDC_AUDIENCE: "forge",
        OIDC_CLIENT_ID: "forge-backoffice",
        BACKOFFICE_API_URL: `http://127.0.0.1:${UPSTREAM}`,
      },
    },
    {
      command: "go run ./cmd/devidp",
      cwd: "../../tools/devidp",
      url: `http://127.0.0.1:${IDP}/.well-known/openid-configuration`,
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
      env: {
        DEVIDP_ISSUER: `http://127.0.0.1:${IDP}`,
        DEVIDP_ADDR: `:${IDP}`,
        DEVIDP_AUDIENCE: "forge",
        DEVIDP_REDIRECT_URIS: `http://127.0.0.1:${OIDC}/api/auth/callback`,
      },
    },
    {
      command: "node tests/fake-upstream.mjs",
      url: `http://127.0.0.1:${UPSTREAM}/__health`,
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
    },
  ],
});
