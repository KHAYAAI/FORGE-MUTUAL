import createClient from "openapi-fetch";
import type { paths } from "./schema";

// Every call goes to /api/v1 on this origin. The route handler adds the session, so the page never holds identity.
export const api = createClient<paths>({ baseUrl: "/api" });

/** The message to show for a failed call. The API's own message is used when it has one. */
export function errorText(error: unknown, fallback: string): string {
  if (typeof error === "object" && error !== null && "error" in error) {
    const message = (error as { error?: { message?: unknown } }).error?.message;
    if (typeof message === "string" && message !== "") return message;
  }
  return fallback;
}
