"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useState } from "react";
import { RequireApplicant } from "@/components/apply/RequireApplicant";
import { Field, Shell, buttonClass, inputClass } from "@/components/apply/Shell";
import { applyPerson, validSAID } from "@/lib/apply";

export default function ApplyPersonal() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [id, setID] = useState("");
  const [agreed, setAgreed] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (!validSAID(id)) return setError("That is not a valid South African ID number. Check the 13 digits.");
    setBusy(true);
    const res = await applyPerson(name.trim(), id.replace(/\s/g, ""));
    setBusy(false);
    if (!res.ok) return setError(res.message);
    router.push("/apply/status/person");
  }

  return (
    <Shell>
      <p className="text-xs uppercase tracking-widest text-[#6B6860]">Personal account</p>
      <h1 className="mt-2 text-3xl font-normal tracking-tight">Tell us who you are</h1>
      <p className="mt-3 text-sm text-[#3B3A36]">
        Next, you will verify your identity with our verification partner: your ID and a selfie, about two minutes.
      </p>
      <div className="mt-8">
        <RequireApplicant next="/apply/personal">
          <form onSubmit={submit} className="space-y-6" aria-label="Personal account application" noValidate>
            <Field label="Full legal name" hint="As it appears on your ID">
              <input className={inputClass} value={name} onChange={(e) => setName(e.target.value)} required maxLength={200} autoComplete="name" />
            </Field>
            <Field label="South African ID number" hint="13 digits">
              <input
                className={`${inputClass} font-mono tracking-wider`}
                value={id}
                onChange={(e) => setID(e.target.value)}
                required
                inputMode="numeric"
                autoComplete="off"
                maxLength={16}
              />
            </Field>
            <label className="flex items-start gap-3 text-sm">
              <input type="checkbox" className="mt-1" checked={agreed} onChange={(e) => setAgreed(e.target.checked)} required />
              <span>
                I agree to the account terms, and to Forge and its verification partner processing my identity
                document and selfie to verify who I am.
              </span>
            </label>
            {error ? (
              <p role="alert" className="rounded-md bg-[#F6E7E1] px-3 py-2 text-sm text-[#7A2E17]">
                {error}
              </p>
            ) : null}
            <button type="submit" className={buttonClass} disabled={busy || !agreed || !name.trim()}>
              {busy ? "Sending…" : "Continue"} <span aria-hidden="true">→</span>
            </button>
          </form>
        </RequireApplicant>
      </div>
    </Shell>
  );
}
