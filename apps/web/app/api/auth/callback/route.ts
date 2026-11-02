import { NextRequest } from "next/server";
import { config } from "@/lib/config";
import { ACCESS_COOKIE, PKCE_COOKIE, exchangeCode, redirectWith, verifyAccessToken } from "@/lib/oidc";

// Receives the provider's redirect, checks the state, exchanges the code with the PKCE verifier, verifies the token, and
// keeps it in an httpOnly cookie. Any failure goes back to sign-in with no session.
export async function GET(req: NextRequest) {
  const cfg = config();
  if (cfg.mode !== "oidc") return new Response("Not found", { status: 404 });
  const q = req.nextUrl.searchParams;
  const secure = cfg.production ? "; Secure" : "";
  const fail = (reason: string) =>
    redirectWith(new URL(`/sign-in?error=${encodeURIComponent(reason)}`, cfg.publicUrl).toString(), [
      `${PKCE_COOKIE}=; Path=/api/auth; Max-Age=0`,
    ]);
  if (q.get("error")) return fail("the sign-in was not completed");
  const raw = req.cookies.get(PKCE_COOKIE)?.value;
  if (!raw) return fail("the sign-in expired; start again");
  let pending: { state: string; verifier: string };
  try {
    pending = JSON.parse(decodeURIComponent(raw));
  } catch {
    return fail("the sign-in could not be read; start again");
  }
  const code = q.get("code");
  if (!code || q.get("state") !== pending.state) return fail("the sign-in did not match; start again");
  try {
    const token = await exchangeCode(cfg, code, pending.verifier);
    await verifyAccessToken(cfg, token.access_token);
    return redirectWith(new URL("/", cfg.publicUrl).toString(), [
      `${ACCESS_COOKIE}=${token.access_token}; Path=/; HttpOnly; SameSite=Strict; Max-Age=${token.expires_in}${secure}`,
      `${PKCE_COOKIE}=; Path=/api/auth; Max-Age=0`,
    ]);
  } catch {
    return fail("the identity provider's sign-in could not be verified");
  }
}
