import createClient from "openapi-fetch";
import type { paths } from "./schema";

// Every call goes to /api/v1 on this origin. The route handler adds the session, so the page never holds identity.
export const api = createClient<paths>({ baseUrl: "/api" });

/** What the sign-in page says when a sign-in did not use a second factor. */
export const MFA_MESSAGE =
  "Your sign-in did not use a second factor, which every Forge account requires. Sign in again and complete the second step with your authenticator app.";

// A sign-in without a second factor is valid but refused (403, mfa_required). Rather than leave every page showing
// errors, send the person back to sign in with a plain explanation.
api.use({
  async onResponse({ response }) {
    if (response.status !== 403 || typeof window === "undefined") return response;
    const body = (await response.clone().json().catch(() => null)) as { error?: { code?: string } } | null;
    if (body?.error?.code === "mfa_required") {
      window.location.assign(`/sign-in?error=${encodeURIComponent(MFA_MESSAGE)}`);
    }
    return response;
  },
});

/** The message to show for a failed call. The API's own message is used when it has one. */
export function errorText(error: unknown, fallback: string): string {
  if (typeof error === "object" && error !== null && "error" in error) {
    const message = (error as { error?: { message?: unknown } }).error?.message;
    if (typeof message === "string" && message !== "") return message;
  }
  return fallback;
}
