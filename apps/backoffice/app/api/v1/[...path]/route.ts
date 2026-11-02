import { cookies } from "next/headers";
import { SESSION_COOKIE, parseStaff } from "@/lib/session";

// Forwards a page's request to backoffice-api with the staff session's name and roles. Only content-type is copied
// from the page, so a page cannot set the identity headers itself.
async function forward(req: Request, ctx: { params: Promise<{ path: string[] }> }) {
  const session = parseStaff((await cookies()).get(SESSION_COOKIE)?.value);
  if (!session) {
    return Response.json({ error: { code: "unauthenticated", message: "sign in to continue" } }, { status: 401 });
  }
  const { path } = await ctx.params;
  const base = process.env.BACKOFFICE_API_URL ?? "http://127.0.0.1:8500";
  const url = new URL(`/v1/${path.map(encodeURIComponent).join("/")}`, base);
  url.search = new URL(req.url).search;

  const headers = new Headers();
  const contentType = req.headers.get("content-type");
  if (contentType) headers.set("content-type", contentType);
  headers.set("x-forge-staff", session.name);
  headers.set("x-forge-roles", session.roles.join(","));

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
      { error: { code: "unavailable", message: "the back office service is not reachable; try again shortly" } },
      { status: 503 },
    );
  }
  return new Response(await upstream.text(), {
    status: upstream.status,
    headers: { "content-type": upstream.headers.get("content-type") ?? "application/json" },
  });
}

export { forward as GET, forward as POST };
