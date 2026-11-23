// The customer front door's calls. Every request goes through this app's /api/v1 route, which adds who is signed in.

export type Kind = "person" | "business";

export type Application = {
  kind: Kind;
  state: "in_progress" | "approved" | "declined" | "failed";
  next: "verify" | "wait" | "";
  steps: string[];
  verify?: { kind: Kind; url: string }[];
  party_id?: string;
};

export type Director = { legal_name: string; national_id: string; ownership_bps: number; is_ubo: boolean };

export const SECTORS: { code: string; label: string }[] = [
  { code: "software_saas", label: "Software as a service" },
  { code: "software_services", label: "Software development and services" },
  { code: "agency_digital", label: "Digital agency" },
  { code: "data_analytics", label: "Data and analytics" },
];

export type Result<T> = { ok: true; value: T } | { ok: false; status: number; message: string };

async function call<T>(path: string, init?: RequestInit): Promise<Result<T>> {
  let res: Response;
  try {
    res = await fetch(`/api/v1${path}`, { cache: "no-store", ...init });
  } catch {
    return { ok: false, status: 0, message: "We could not reach Forge. Check your connection and try again." };
  }
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    const message = (body as { error?: { message?: string } } | null)?.error?.message ?? "Something went wrong. Try again.";
    return { ok: false, status: res.status, message };
  }
  return { ok: true, value: body as T };
}

export function getApplication(kind: Kind) {
  return call<Application>(`/applications/${kind}`);
}

export function applyPerson(legal_name: string, national_id: string) {
  return call<Application>("/applications/person", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ legal_name, national_id }),
  });
}

export function applyBusiness(body: { legal_name: string; registration_number: string; sector_code: string; directors: Director[] }) {
  return call<Application>("/applications/business", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** A South African identity number: 13 digits, a real date of birth, and the Luhn check digit. */
export function validSAID(raw: string): boolean {
  const id = raw.replace(/\s/g, "");
  if (!/^\d{13}$/.test(id)) return false;
  let sum = 0;
  for (let i = 0; i < 13; i++) {
    let d = Number(id[i]);
    if (i % 2 === 1) {
      d *= 2;
      if (d > 9) d -= 9;
    }
    sum += d;
  }
  if (sum % 10 !== 0) return false;
  const mm = Number(id.slice(2, 4));
  const dd = Number(id.slice(4, 6));
  return mm >= 1 && mm <= 12 && dd >= 1 && dd <= 31;
}
