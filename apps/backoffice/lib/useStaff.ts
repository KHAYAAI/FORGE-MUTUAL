"use client";

import { useEffect, useState } from "react";
import type { Role } from "./session";

export type Staff = { signedIn: true; name: string; roles: Role[] } | { signedIn: false };

/**
 * Who is signed in, and their roles. It is read again when the path changes, so a sign-in shows at once.
 * The page uses the roles only to show or hide actions; the server still decides.
 */
export function useStaff(path: string): Staff | null {
  const [staff, setStaff] = useState<Staff | null>(null);
  useEffect(() => {
    let live = true;
    fetch("/api/session", { cache: "no-store" })
      .then(async (res) => {
        const body = (await res.json()) as { signedIn: boolean; name?: string; roles?: Role[] };
        if (!live) return;
        setStaff(body.signedIn && body.name && body.roles ? { signedIn: true, name: body.name, roles: body.roles } : { signedIn: false });
      })
      .catch(() => live && setStaff({ signedIn: false }));
    return () => {
      live = false;
    };
  }, [path]);
  return staff;
}

export function hasRole(staff: Staff | null, role: Role): boolean {
  return staff !== null && staff.signedIn && staff.roles.includes(role);
}
