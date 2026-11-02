// The staff session is an httpOnly cookie set by /api/session. Only the route handlers read it.
export const SESSION_COOKIE = "forge_staff";

export const ROLES = ["compliance", "operations"] as const;
export type Role = (typeof ROLES)[number];
export type StaffSession = { name: string; roles: Role[] };

/** Checks a session the browser supplied before it is stored. A session needs a name and at least one known role. */
export function validStaff(input: unknown): StaffSession | null {
  if (typeof input !== "object" || input === null) return null;
  const { name, roles } = input as Record<string, unknown>;
  if (typeof name !== "string" || name.trim() === "" || name.trim().length > 64) return null;
  if (!Array.isArray(roles)) return null;
  const known = roles.filter((r): r is Role => typeof r === "string" && (ROLES as readonly string[]).includes(r));
  const unique = Array.from(new Set(known));
  if (unique.length === 0 || unique.length !== roles.length) return null;
  return { name: name.trim(), roles: unique };
}

/** Reads the session from the cookie value. A malformed value is no session. */
export function parseStaff(raw: string | undefined): StaffSession | null {
  if (!raw) return null;
  try {
    return validStaff(JSON.parse(raw));
  } catch {
    return null;
  }
}
