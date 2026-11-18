import { config } from "@/lib/config";
import { PKCE_COOKIE, authorizeUrl, discover, pkcePair, randomState, redirectWith } from "@/lib/oidc";

// Starts the sign-in: a state and a PKCE pair are kept in a short-lived cookie, and the browser goes to the identity
// provider. Only available when the mode is oidc.
export async function GET() {
  const cfg = config();
  if (cfg.mode !== "oidc") return new Response("Not found", { status: 404 });
  const doc = await discover(cfg.issuer);
  const state = randomState();
  const { verifier, challenge } = pkcePair();
  const cookie = `${PKCE_COOKIE}=${encodeURIComponent(JSON.stringify({ state, verifier }))}; Path=/api/auth; HttpOnly; SameSite=Lax; Max-Age=600${cfg.production ? "; Secure" : ""}`;
  return redirectWith(authorizeUrl(cfg, doc, state, challenge), [cookie]);
}
