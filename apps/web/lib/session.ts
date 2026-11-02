// The session is an httpOnly cookie set by /api/session. Only the route handlers read it; the page code never does.
export const SESSION_COOKIE = "forge_session";

export type Session = { party: string; actor: string };

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

/** Checks a session the browser supplied before it is stored. */
export function validSession(input: unknown): Session | null {
  if (typeof input !== "object" || input === null) return null;
  const { party, actor } = input as Record<string, unknown>;
  if (typeof party !== "string" || !UUID.test(party)) return null;
  if (typeof actor !== "string" || actor.trim() === "" || actor.length > 64) return null;
  return { party, actor: actor.trim() };
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
