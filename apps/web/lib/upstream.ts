// The one call this app's server makes to its API.
//
// In production the API serves only callers it can authenticate by certificate (mutual TLS, pkg/serve). So this server
// presents its own certificate and trusts only the platform's certificate authority, with the same three settings the
// Go services use:
//
//   TLS_CERT_FILE   this app's certificate (PEM)
//   TLS_KEY_FILE    its private key (PEM)
//   TLS_CA_FILE     the authority that signs every service certificate (PEM)
//
// Production refuses to start without them, and refuses an API address that is not https. In development all three may be
// absent and the API is reached over plain HTTP.
//
// Node's fetch cannot present a client certificate without another dependency, so this uses node:https.
import { readFileSync, statSync } from "node:fs";
import http from "node:http";
import https from "node:https";

type Env = Record<string, string | undefined>;

export type UpstreamResponse = { status: number; contentType: string; body: string };

/** Refuses a configuration that production must not run with. Called when the server starts, and on every call. */
export function upstreamPolicy(base: string, env: Env = process.env): void {
  const files = [env.TLS_CERT_FILE, env.TLS_KEY_FILE, env.TLS_CA_FILE];
  const set = files.filter(Boolean).length;
  if (set !== 0 && set !== 3) {
    throw new Error("TLS_CERT_FILE, TLS_KEY_FILE and TLS_CA_FILE must be set together");
  }
  if (env.FORGE_ENV !== "production") return;
  if (set !== 3) {
    throw new Error("production requires TLS_CERT_FILE, TLS_KEY_FILE and TLS_CA_FILE: the API accepts only mutually authenticated callers");
  }
  if (!base.startsWith("https://")) {
    throw new Error("production: the API address must be https");
  }
}

// The agent holds the certificate. It is rebuilt when any of the three files changes, so a certificate that is renewed
// on disk is picked up without a restart.
let cached: { agent: https.Agent; stamp: string } | undefined;

function tlsAgent(env: Env): https.Agent | undefined {
  const { TLS_CERT_FILE: cert, TLS_KEY_FILE: key, TLS_CA_FILE: ca } = env;
  if (!cert || !key || !ca) return undefined;
  const stamp = [cert, key, ca].map((f) => `${f}:${statSync(f).mtimeMs}`).join("|");
  if (!cached || cached.stamp !== stamp) {
    cached?.agent.destroy();
    cached = {
      stamp,
      agent: new https.Agent({ cert: readFileSync(cert), key: readFileSync(key), ca: readFileSync(ca), minVersion: "TLSv1.2", keepAlive: true }),
    };
  }
  return cached.agent;
}

/** Sends one request to the API and returns its status, content type and body. Rejects if the API cannot be reached. */
export function callUpstream(
  url: URL,
  init: { method: string; headers: Record<string, string>; body?: string },
  env: Env = process.env,
): Promise<UpstreamResponse> {
  upstreamPolicy(url.origin, env);
  const secure = url.protocol === "https:";
  const agent = secure ? tlsAgent(env) : undefined;
  return new Promise((resolve, reject) => {
    const req = (secure ? https : http).request(
      url,
      { method: init.method, headers: init.headers, agent, timeout: 30_000 },
      (res) => {
        const chunks: Buffer[] = [];
        res.on("data", (c: Buffer) => chunks.push(c));
        res.on("end", () =>
          resolve({
            status: res.statusCode ?? 502,
            contentType: res.headers["content-type"] ?? "application/json",
            body: Buffer.concat(chunks).toString("utf8"),
          }),
        );
        res.on("error", reject);
      },
    );
    req.on("timeout", () => req.destroy(new Error("the API did not answer in time")));
    req.on("error", reject);
    if (init.body !== undefined) req.write(init.body);
    req.end();
  });
}
