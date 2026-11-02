"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useState } from "react";
import Notice from "@/components/Notice";
import { errorText } from "@/lib/api/client";
import { ROLES, type Role } from "@/lib/session";

export default function SignInPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [roles, setRoles] = useState<Role[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function toggle(role: Role, on: boolean) {
    setRoles((prev) => (on ? [...prev, role] : prev.filter((r) => r !== role)));
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const res = await fetch("/api/session", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ name: name.trim(), roles }),
    });
    setBusy(false);
    if (!res.ok) {
      const body = await res.json().catch(() => null);
      return setError(errorText(body, "We could not sign you in. Check your name and choose a role."));
    }
    router.push("/cases");
    router.refresh();
  }

  return (
    <section className="mx-auto max-w-md space-y-6">
      <h1 className="text-2xl font-semibold">Sign in</h1>
      <Notice tone="info">
        Development sign-in: your name and the roles you hold. Production sign-in is not built yet.
      </Notice>
      {error ? <Notice tone="error">{error}</Notice> : null}
      <form onSubmit={submit} className="space-y-4" aria-label="Sign in">
        <label className="block text-sm">
          <span className="font-medium">Your name</span>
          <input
            name="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            maxLength={64}
            autoComplete="name"
            className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2 text-sm"
          />
        </label>
        <fieldset className="space-y-2 text-sm">
          <legend className="font-medium">Your roles</legend>
          {ROLES.map((role) => (
            <label key={role} className="flex items-center gap-2">
              <input
                type="checkbox"
                name={role}
                checked={roles.includes(role)}
                onChange={(e) => toggle(role, e.target.checked)}
              />
              <span>{role === "compliance" ? "Compliance (closes cases)" : "Operations (decides payments, repairs, breaks)"}</span>
            </label>
          ))}
        </fieldset>
        <button
          type="submit"
          disabled={busy}
          className="rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white hover:bg-stone-800 disabled:opacity-50"
        >
          {busy ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </section>
  );
}
