import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { config } from "@/lib/config";
import { ACCESS_COOKIE, verifyAccessToken } from "@/lib/oidc";
import { SESSION_COOKIE, knownRoles, parseStaff, validStaff } from "@/lib/session";

// Who the staff member is. In oidc mode it is read from the verified token; in dev mode from the dev cookie.
async function current() {
  const cfg = config();
  const jar = await cookies();
  if (cfg.mode === "oidc") {
    const token = jar.get(ACCESS_COOKIE)?.value;
    if (!token) return null;
    try {
      const p = await verifyAccessToken(cfg, token);
      const roles = knownRoles(p.forge_roles);
      const name = [p.name, p.email, p.sub].find((v): v is string => typeof v === "string" && v.trim() !== "") ?? "";
      if (roles.length === 0 || name === "") return null;
      return { name: name.slice(0, 64), roles };
    } catch {
      return null;
    }
  }
  return parseStaff(jar.get(SESSION_COOKIE)?.value);
}

// The session the browser may see: the name and roles, never a token or the cookie value.
export async function GET() {
  const who = await current();
  if (!who) return NextResponse.json({ signedIn: false }, { status: 401 });
  return NextResponse.json({ signedIn: true, name: who.name, roles: who.roles });
}

// Dev sign-in. It does not exist in oidc mode.
export async function POST(req: Request) {
  const cfg = config();
  if (cfg.mode !== "dev") return new NextResponse("Not found", { status: 404 });
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: { code: "invalid_argument", message: "send a JSON body" } }, { status: 400 });
  }
  const session = validStaff(body);
  if (!session) {
    return NextResponse.json(
      { error: { code: "invalid_argument", message: "enter your name and choose at least one role" } },
      { status: 400 },
    );
  }
  const res = NextResponse.json({ signedIn: true, name: session.name, roles: session.roles });
  res.cookies.set(SESSION_COOKIE, JSON.stringify(session), {
    httpOnly: true,
    sameSite: "strict",
    secure: cfg.production,
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
