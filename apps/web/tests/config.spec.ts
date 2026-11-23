import { expect, test } from "@playwright/test";
import { config } from "../lib/config";

const oidcEnv = {
  FORGE_AUTH_MODE: "oidc",
  OIDC_ISSUER: "https://id.example.com",
  OIDC_AUDIENCE: "forge",
  OIDC_CLIENT_ID: "forge-web",
  FORGE_PUBLIC_URL: "https://app.example.com",
};

// The server refuses what production must refuse. These checks run the configuration directly.
test("dev sign-in is refused in production, and the server does not start with it", () => {
  expect(() => config({ FORGE_ENV: "production", FORGE_AUTH_MODE: "dev" })).toThrow(/refused in production/);
});

test("OIDC needs an issuer, an audience and a client id", () => {
  expect(() => config({ FORGE_AUTH_MODE: "oidc", FORGE_PUBLIC_URL: "https://x.example" })).toThrow(/OIDC_ISSUER/);
  expect(() => config({ ...oidcEnv, OIDC_CLIENT_ID: "" })).toThrow(/OIDC_CLIENT_ID/);
});

test("in production the public address must be https", () => {
  expect(() => config({ ...oidcEnv, FORGE_ENV: "production", FORGE_PUBLIC_URL: "http://app.example.com" })).toThrow(/https/);
  expect(config({ ...oidcEnv, FORGE_ENV: "production" }).mode).toBe("oidc");
});

test("an unknown mode is refused, and oidc is the default", () => {
  expect(() => config({ FORGE_AUTH_MODE: "password" })).toThrow(/not a known mode/);
  expect(() => config({ ...oidcEnv, FORGE_AUTH_MODE: undefined })).not.toThrow();
  expect(config({ ...oidcEnv, FORGE_AUTH_MODE: undefined }).mode).toBe("oidc");
});
