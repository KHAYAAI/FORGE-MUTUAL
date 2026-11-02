import { createHash, randomBytes } from "node:crypto";
import { createRemoteJWKSet, jwtVerify, type JWTPayload } from "jose";
import type { Config } from "./config";

// OpenID Connect for the customer app: authorization code with PKCE, and access token verification against the issuer's
// keys. Discovery and key sets are cached briefly. The API verifies the token again; this check fails early and keeps the
// session honest.

export const PKCE_COOKIE = "forge_pkce";
export const ACCESS_COOKIE = "forge_access";

export type Discovery = {
  issuer: string;
  authorization_endpoint: string;
  token_endpoint: string;
  jwks_uri: string;
};

const CACHE_MS = 10 * 60 * 1000;
const discoveryCache = new Map<string, { doc: Discovery; at: number }>();
const keySets = new Map<string, ReturnType<typeof createRemoteJWKSet>>();

export async function discover(issuer: string): Promise<Discovery> {
  const hit = discoveryCache.get(issuer);
  if (hit && Date.now() - hit.at < CACHE_MS) return hit.doc;
  const res = await fetch(`${issuer.replace(/\/$/, "")}/.well-known/openid-configuration`, { cache: "no-store" });
  if (!res.ok) throw new Error(`discovery failed: ${res.status}`);
  const doc = (await res.json()) as Discovery;
  if (doc.issuer !== issuer) throw new Error("discovery names another issuer");
  discoveryCache.set(issuer, { doc, at: Date.now() });
  return doc;
}

function keysFor(doc: Discovery) {
  let keys = keySets.get(doc.jwks_uri);
  if (!keys) {
    keys = createRemoteJWKSet(new URL(doc.jwks_uri));
    keySets.set(doc.jwks_uri, keys);
  }
  return keys;
}

/** A new PKCE verifier and its S256 challenge. */
export function pkcePair() {
  const verifier = randomBytes(32).toString("base64url");
  const challenge = createHash("sha256").update(verifier).digest("base64url");
  return { verifier, challenge };
}

export function randomState(): string {
  return randomBytes(24).toString("base64url");
}

export function callbackUrl(cfg: Config): string {
  return `${cfg.publicUrl.replace(/\/$/, "")}/api/auth/callback`;
}

export async function verifyAccessToken(cfg: Config, token: string): Promise<JWTPayload> {
  const doc = await discover(cfg.issuer);
  const { payload } = await jwtVerify(token, keysFor(doc), {
    issuer: cfg.issuer,
    audience: cfg.audience,
    algorithms: ["RS256", "ES256"],
  });
  return payload;
}

export async function exchangeCode(
  cfg: Config,
  code: string,
  verifier: string,
): Promise<{ access_token: string; expires_in: number }> {
  const doc = await discover(cfg.issuer);
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    code,
    redirect_uri: callbackUrl(cfg),
    client_id: cfg.clientId,
    code_verifier: verifier,
  });
  if (cfg.clientSecret) body.set("client_secret", cfg.clientSecret);
  const res = await fetch(doc.token_endpoint, {
    method: "POST",
    headers: { "content-type": "application/x-www-form-urlencoded" },
    body,
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`token exchange failed: ${res.status}`);
  const json = (await res.json()) as { access_token?: string; expires_in?: number };
  if (!json.access_token) throw new Error("token exchange returned no access token");
  return { access_token: json.access_token, expires_in: json.expires_in ?? 900 };
}

export function authorizeUrl(cfg: Config, doc: Discovery, state: string, challenge: string): string {
  const u = new URL(doc.authorization_endpoint);
  u.search = new URLSearchParams({
    response_type: "code",
    client_id: cfg.clientId,
    redirect_uri: callbackUrl(cfg),
    scope: "openid profile email",
    state,
    code_challenge: challenge,
    code_challenge_method: "S256",
  }).toString();
  return u.toString();
}

/** A redirect whose headers can be added to. Response.redirect() returns immutable headers, so cookies cannot go on it. */
export function redirectWith(location: string, setCookies: string[] = []): Response {
  const headers = new Headers({ location });
  for (const c of setCookies) headers.append("set-cookie", c);
  return new Response(null, { status: 302, headers });
}
