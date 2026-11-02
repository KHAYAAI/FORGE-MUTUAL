import { cookies } from "next/headers";
import { SESSION_COOKIE, parseSession } from "@/lib/session";

// Forwards a page's request to workspace-api with the session's identity. Only the headers listed here are copied, so a
// page cannot set X-Forge-Party or X-Forge-Actor itself.
const FORWARDED = ["content-type", "idempotency-key"];

async function forward(req: Request, ctx: { params: Promise<{ path: string[] }> }) {
  const session = parseSession((await cookies()).get(SESSION_COOKIE)?.value);
  if (!session) {
    return Response.json({ error: { code: "unauthenticated", message: "sign in to continue" } }, { status: 401 });
  }
  const { path } = await ctx.params;
  const base = process.env.WORKSPACE_API_URL ?? "http://127.0.0.1:8300";
  const url = new URL(`/v1/${path.map(encodeURIComponent).join("/")}`, base);
  url.search = new URL(req.url).search;

  const headers = new Headers();
  for (const name of FORWARDED) {
    const value = req.headers.get(name);
    if (value) headers.set(name, value);
  }
  headers.set("x-forge-party", session.party);
  headers.set("x-forge-actor", session.actor);

  const hasBody = req.method !== "GET" && req.method !== "HEAD";
  let upstream: Response;
  try {
    upstream = await fetch(url, {
      method: req.method,
      headers,
      body: hasBody ? await req.text() : undefined,
      cache: "no-store",
    });
  } catch {
    // The page can say what happened, rather than failing without a message.
    return Response.json(
      { error: { code: "unavailable", message: "the workspace service is not reachable; try again shortly" } },
      { status: 503 },
    );
  }
  return new Response(await upstream.text(), {
    status: upstream.status,
    headers: { "content-type": upstream.headers.get("content-type") ?? "application/json" },
  });
}

export { forward as GET, forward as POST };
