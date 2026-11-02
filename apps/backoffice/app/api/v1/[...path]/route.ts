import { cookies } from "next/headers";
import { config } from "@/lib/config";
import { ACCESS_COOKIE } from "@/lib/oidc";
import { SESSION_COOKIE, parseStaff } from "@/lib/session";

// Forwards a page's request to backoffice-api. In oidc mode the access token is sent as a bearer token, and the API
// verifies it. In dev mode the staff member's name and roles are sent as headers. Only content-type and the idempotency
// key are copied from the page, so a page cannot set identity itself.
async function forward(req: Request, ctx: { params: Promise<{ path: string[] }> }) {
  const cfg = config();
  const jar = await cookies();
  const headers = new Headers();
  const contentType = req.headers.get("content-type");
  if (contentType) headers.set("content-type", contentType);

  if (cfg.mode === "oidc") {
    const token = jar.get(ACCESS_COOKIE)?.value;
    if (!token) return unauthenticated();
    headers.set("authorization", `Bearer ${token}`);
  } else {
    const session = parseStaff(jar.get(SESSION_COOKIE)?.value);
    if (!session) return unauthenticated();
    headers.set("x-forge-staff", session.name);
    headers.set("x-forge-roles", session.roles.join(","));
  }

  const { path } = await ctx.params;
  const url = new URL(`/v1/${path.map(encodeURIComponent).join("/")}`, process.env.BACKOFFICE_API_URL ?? "http://127.0.0.1:8500");
  url.search = new URL(req.url).search;

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

function unauthenticated() {
  return Response.json({ error: { code: "unauthenticated", message: "sign in to continue" } }, { status: 401 });
}

export { forward as GET, forward as POST };
