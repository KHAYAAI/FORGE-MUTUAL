import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { config } from "@/lib/config";
import { SESSION_COOKIE, parseSession } from "@/lib/session";
import { callUpstream } from "@/lib/upstream";

// Turns an approved applicant into a signed-in customer. The party comes from the applicant's own approved
// application, read from the API as that applicant; the browser cannot name it.
//
// Development sign-in only. With the identity provider, the customer's party is a claim in their token, which the
// provider issues once the party is recorded against their user; they sign in again to get it.
export async function POST() {
  const cfg = config();
  if (cfg.mode !== "dev") {
    return NextResponse.json(
      { error: { code: "sign_in_again", message: "Your account is open. Sign in again to see it." } },
      { status: 409 },
    );
  }
  const jar = await cookies();
  const session = parseSession(jar.get(SESSION_COOKIE)?.value);
  if (!session?.applicant) {
    return NextResponse.json({ error: { code: "unauthenticated", message: "sign in to continue" } }, { status: 401 });
  }
  const base = process.env.WORKSPACE_API_URL ?? "http://127.0.0.1:8300";
  let party = "";
  for (const kind of ["business", "person"]) {
    try {
      const res = await callUpstream(new URL(`/v1/applications/${kind}`, base), {
        method: "GET",
        headers: { "x-forge-applicant": session.applicant, "x-forge-actor": session.actor },
      });
      if (res.status !== 200) continue;
      const app = JSON.parse(res.body) as { state?: string; party_id?: string };
      if (app.state === "approved" && app.party_id) {
        party = app.party_id;
        break;
      }
    } catch {
      return NextResponse.json({ error: { code: "unavailable", message: "try again shortly" } }, { status: 503 });
    }
  }
  if (!party) {
    return NextResponse.json({ error: { code: "not_approved", message: "your application is not approved yet" } }, { status: 409 });
  }
  const res = NextResponse.json({ signedIn: true, party });
  res.cookies.set(SESSION_COOKIE, JSON.stringify({ ...session, party }), {
    httpOnly: true,
    sameSite: "strict",
    secure: cfg.production,
    path: "/",
    maxAge: 60 * 60 * 8,
  });
  return res;
}
