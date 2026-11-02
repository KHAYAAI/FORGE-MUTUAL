import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { SESSION_COOKIE, parseSession, validSession } from "@/lib/session";

// The session the browser may see: who it is, never the cookie value.
export async function GET() {
  const session = parseSession((await cookies()).get(SESSION_COOKIE)?.value);
  if (!session) return NextResponse.json({ signedIn: false }, { status: 401 });
  return NextResponse.json({ signedIn: true, party: session.party, actor: session.actor });
}

export async function POST(req: Request) {
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
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: 60 * 60 * 8,
  });
  return res;
}

export async function DELETE() {
  const res = NextResponse.json({ signedIn: false });
  res.cookies.delete(SESSION_COOKIE);
  return res;
}
