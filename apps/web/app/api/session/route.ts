import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { config } from "@/lib/config";
import { ACCESS_COOKIE, verifyAccessToken } from "@/lib/oidc";
import { SESSION_COOKIE, parseSession, validSession } from "@/lib/session";

type Who = { party: string; actor: string };

// Who the browser is signed in as. In oidc mode it is read from the verified token; in dev mode from the dev cookie.
async function current(): Promise<Who | null> {
  const cfg = config();
  const jar = await cookies();
  if (cfg.mode === "oidc") {
    const token = jar.get(ACCESS_COOKIE)?.value;
    if (!token) return null;
    try {
      const p = await verifyAccessToken(cfg, token);
      const party = typeof p.forge_party === "string" ? p.forge_party : "";
      const name = [p.name, p.email, p.sub].find((v): v is string => typeof v === "string" && v.trim() !== "") ?? "";
      return validSession({ party, actor: name.slice(0, 64) });
    } catch {
      return null;
    }
  }
  return parseSession(jar.get(SESSION_COOKIE)?.value);
}

// The session the browser may see: who it is, never a token or the cookie value.
export async function GET() {
  const who = await current();
  if (!who) return NextResponse.json({ signedIn: false }, { status: 401 });
  return NextResponse.json({ signedIn: true, party: who.party, actor: who.actor });
}

// Dev sign-in. It does not exist in oidc mode.
export async function POST(req: Request) {
  if (config().mode !== "dev") return new NextResponse("Not found", { status: 404 });
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: { code: "invalid_argument", message: "send a JSON body" } }, { status: 400 });
  }
  const session = validSession(body);
  if (!session) {
    return NextResponse.json(
      { error: { code: "invalid_argument", message: "enter a party id (a UUID) and your name" } },
      { status: 400 },
    );
  }
  const res = NextResponse.json({ signedIn: true, party: session.party, actor: session.actor });
  res.cookies.set(SESSION_COOKIE, JSON.stringify(session), {
    httpOnly: true,
    sameSite: "strict",
    secure: config().production,
    path: "/",
    maxAge: 60 * 60 * 8,
  });
  return res;
}

// Sign out: clears both kinds of session.
export async function DELETE() {
  const res = NextResponse.json({ signedIn: false });
  res.cookies.delete(SESSION_COOKIE);
  res.cookies.delete(ACCESS_COOKIE);
  return res;
}
