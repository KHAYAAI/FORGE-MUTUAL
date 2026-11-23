import { callUpstream } from "@/lib/upstream";

const TOKEN = /^[a-z2-7]{16,64}$/;

/** Forwards a payer's checkout call to workspace-api with no identity at all: the token is the only credential. */
export async function forwardCheckout(method: "GET" | "POST", token: string, action: string, body: string | undefined) {
  if (!TOKEN.test(token)) return Response.json({ error: { code: "not_found", message: "no such payment" } }, { status: 404 });
  const path = `/v1/checkout/${token}${action ? `/${action}` : ""}`;
  const url = new URL(path, process.env.WORKSPACE_API_URL ?? "http://127.0.0.1:8300");
  try {
    const up = await callUpstream(url, { method, headers: body ? { "content-type": "application/json" } : {}, body });
    return new Response(up.body, { status: up.status, headers: { "content-type": up.contentType } });
  } catch {
    return Response.json({ error: { code: "unavailable", message: "payments are not reachable; try again shortly" } }, { status: 503 });
  }
}
