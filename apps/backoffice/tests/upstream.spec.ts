import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import https from "node:https";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test } from "@playwright/test";
import { callUpstream, upstreamPolicy } from "../lib/upstream";

// The app's server reaches its API over mutual TLS. These tests run the call directly, against a server that requires a
// client certificate, with certificates from the same script the local stack uses.

const pkiScript = join(__dirname, "..", "..", "..", "deploy", "compose", "pki.sh");
const tls = { TLS_CERT_FILE: "/pki/app.crt", TLS_KEY_FILE: "/pki/app.key", TLS_CA_FILE: "/pki/ca.crt" };

test("production refuses to run without a certificate, or with an API address that is not https", () => {
  expect(() => upstreamPolicy("https://api.internal", { FORGE_ENV: "production" })).toThrow(/production requires TLS_CERT_FILE/);
  expect(() => upstreamPolicy("http://api.internal", { FORGE_ENV: "production", ...tls })).toThrow(/must be https/);
  expect(() => upstreamPolicy("https://api.internal", { FORGE_ENV: "production", ...tls })).not.toThrow();
});

test("the three TLS settings come together or not at all", () => {
  expect(() => upstreamPolicy("http://127.0.0.1:8500", { TLS_CERT_FILE: "/pki/app.crt" })).toThrow(/set together/);
  expect(() => upstreamPolicy("http://127.0.0.1:8500", {})).not.toThrow();
});

test.describe("against a server that requires a client certificate", () => {
  let dir: string;
  let other: string;
  let server: https.Server;
  let base: string;
  const seen: string[] = [];

  test.beforeAll(async () => {
    dir = mkdtempSync(join(tmpdir(), "forge-upstream-"));
    other = mkdtempSync(join(tmpdir(), "forge-upstream-other-"));
    execFileSync(pkiScript, [dir, "svc-backoffice-web", "svc-api"]);
    execFileSync(pkiScript, [other, "svc-backoffice-web"]); // the same name, from an authority the server does not trust
    server = https.createServer(
      {
        cert: readFileSync(join(dir, "svc-api.crt")),
        key: readFileSync(join(dir, "svc-api.key")),
        ca: readFileSync(join(dir, "ca.crt")),
        requestCert: true,
        rejectUnauthorized: true,
      },
      (req, res) => {
        const peer = (req.socket as import("node:tls").TLSSocket).getPeerCertificate();
        seen.push(String(peer.subjectaltname));
        let body = "";
        req.on("data", (c) => (body += c));
        req.on("end", () => {
          res.writeHead(201, { "content-type": "application/json" });
          res.end(JSON.stringify({ method: req.method, path: req.url, staff: req.headers["x-forge-staff"], body }));
        });
      },
    );
    await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
    base = `https://127.0.0.1:${(server.address() as AddressInfo).port}`;
  });

  test.afterAll(async () => {
    await new Promise((resolve) => server.close(resolve));
    rmSync(dir, { recursive: true, force: true });
    rmSync(other, { recursive: true, force: true });
  });

  const env = (from: string) => ({
    TLS_CERT_FILE: join(from, "svc-backoffice-web.crt"),
    TLS_KEY_FILE: join(from, "svc-backoffice-web.key"),
    TLS_CA_FILE: join(dir, "ca.crt"),
  });

  test("the app presents its certificate, and the request arrives whole", async () => {
    const res = await callUpstream(
      new URL("/v1/payments?limit=2", base),
      { method: "POST", headers: { "content-type": "application/json", "x-forge-staff": "Naledi" }, body: '{"amount":"100"}' },
      env(dir),
    );
    expect(res.status).toBe(201);
    expect(res.contentType).toBe("application/json");
    expect(JSON.parse(res.body)).toEqual({ method: "POST", path: "/v1/payments?limit=2", staff: "Naledi", body: '{"amount":"100"}' });
    expect(seen.at(-1)).toContain("URI:spiffe://forge/svc/svc-backoffice-web");
  });

  test("without a certificate the API is not reached", async () => {
    await expect(callUpstream(new URL("/v1/accounts", base), { method: "GET", headers: {} }, { NODE_EXTRA_CA_CERTS: join(dir, "ca.crt") })).rejects.toThrow();
  });

  test("a certificate from another authority is refused, whatever name it carries", async () => {
    await expect(callUpstream(new URL("/v1/accounts", base), { method: "GET", headers: {} }, env(other))).rejects.toThrow();
  });

  test("the app refuses a server whose certificate its authority did not sign", async () => {
    await expect(
      callUpstream(new URL("/v1/accounts", base), { method: "GET", headers: {} }, { ...env(dir), TLS_CA_FILE: join(other, "ca.crt") }),
    ).rejects.toThrow();
  });
});
