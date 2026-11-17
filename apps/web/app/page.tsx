import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { config } from "@/lib/config";
import { ACCESS_COOKIE, verifyAccessToken } from "@/lib/oidc";
import { SESSION_COOKIE, parseSession } from "@/lib/session";

// A customer goes to their accounts; everyone else, to the front door.
export default async function Home() {
  const cfg = config();
  const jar = await cookies();
  let customer = false;
  if (cfg.mode === "oidc") {
    const token = jar.get(ACCESS_COOKIE)?.value;
    if (token) {
      try {
        const p = await verifyAccessToken(cfg, token);
        customer = typeof p.forge_party === "string" && p.forge_party !== "";
      } catch {
        customer = false;
      }
    }
  } else {
    customer = !!parseSession(jar.get(SESSION_COOKIE)?.value)?.party;
  }
  redirect(customer ? "/accounts" : "/apply");
}
