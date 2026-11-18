import { forwardCheckout } from "@/lib/checkout";

// Starting a checkout, or the simulator's result. Only these two actions exist; anything else is not found.
export async function POST(req: Request, ctx: { params: Promise<{ token: string; action: string }> }) {
  const { token, action } = await ctx.params;
  if (action !== "start" && action !== "simulate") return Response.json({ error: { code: "not_found", message: "not found" } }, { status: 404 });
  return forwardCheckout("POST", token, action, await req.text());
}
