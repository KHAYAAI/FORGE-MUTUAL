import { forwardCheckout } from "@/lib/checkout";

// The payer's view of a payment. No sign-in and no identity: only the token goes upstream.
export async function GET(_req: Request, ctx: { params: Promise<{ token: string }> }) {
  const { token } = await ctx.params;
  return forwardCheckout("GET", token, "", undefined);
}
