// The session is an httpOnly cookie set by /api/session. Only the route handlers read it; the page code never does.
export const SESSION_COOKIE = "forge_session";

// party is the customer record, once there is one. applicant is the sign-in identity of someone who may still be
// applying; it is the only key to their application. A session has at least one of them.
export type Session = { party: string; applicant: string; actor: string };

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const SUBJECT = /^[\x21-\x7e]{1,200}$/;

/** Checks a session the browser supplied before it is stored. */
export function validSession(input: unknown): Session | null {
  if (typeof input !== "object" || input === null) return null;
  const { party = "", applicant = "", actor } = input as Record<string, unknown>;
  if (typeof party !== "string" || (party !== "" && !UUID.test(party))) return null;
  if (typeof applicant !== "string" || (applicant !== "" && !SUBJECT.test(applicant))) return null;
  if (party === "" && applicant === "") return null;
  if (typeof actor !== "string" || actor.trim() === "" || actor.length > 64) return null;
  return { party, applicant, actor: actor.trim() };
}

/** The development applicant identity for a name: stable, so signing in again finds the same application. */
export function devApplicant(name: string): string {
  return "dev-" + name.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "").slice(0, 60);
}

/** Reads the session from the cookie value. A malformed value is no session. */
export function parseSession(raw: string | undefined): Session | null {
  if (!raw) return null;
  try {
    return validSession(JSON.parse(raw));
  } catch {
    return null;
  }
}
