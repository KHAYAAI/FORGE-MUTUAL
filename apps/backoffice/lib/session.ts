// The staff session. In oidc mode the roles come from the verified token; in dev mode the dev cookie carries them.
// Cookie names are distinct from the customer app's, because cookies are scoped to the host, not the port.
export const SESSION_COOKIE = "forge_staff";

export const ROLES = ["compliance", "operations"] as const;
export type Role = (typeof ROLES)[number];
export type StaffSession = { name: string; roles: Role[] };

/** The roles this app knows, from a claim that may hold any roles the provider adds. */
export function knownRoles(claim: unknown): Role[] {
  const list = Array.isArray(claim) ? claim : typeof claim === "string" ? claim.split(/\s+/) : [];
  const known = list.filter((r): r is Role => typeof r === "string" && (ROLES as readonly string[]).includes(r));
  return Array.from(new Set(known));
}

/** Checks a session the browser supplied before it is stored. A session needs a name and at least one known role. */
export function validStaff(input: unknown): StaffSession | null {
  if (typeof input !== "object" || input === null) return null;
  const { name, roles } = input as Record<string, unknown>;
  if (typeof name !== "string" || name.trim() === "" || name.trim().length > 64) return null;
  if (!Array.isArray(roles)) return null;
  const known = knownRoles(roles);
  if (known.length === 0 || known.length !== new Set(roles).size) return null;
  return { name: name.trim(), roles: known };
}

/** Reads the dev session from its cookie value. A malformed value is no session. */
export function parseStaff(raw: string | undefined): StaffSession | null {
  if (!raw) return null;
  try {
    return validStaff(JSON.parse(raw));
  } catch {
    return null;
  }
}
