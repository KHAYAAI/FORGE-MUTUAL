"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useState } from "react";
import { RequireApplicant } from "@/components/apply/RequireApplicant";
import { Field, Panel, Shell, buttonClass, eyebrowClass, inputClass, titleClass } from "@/components/apply/Shell";
import { type Director, SECTORS, applyBusiness, validSAID } from "@/lib/apply";

type Row = { name: string; id: string; percent: string; ubo: boolean };
const empty: Row = { name: "", id: "", percent: "", ubo: false };

export default function ApplyBusiness() {
  const router = useRouter();
  const [company, setCompany] = useState("");
  const [reg, setReg] = useState("");
  const [sector, setSector] = useState(SECTORS[0].code);
  const [rows, setRows] = useState<Row[]>([{ ...empty }]);
  const [agreed, setAgreed] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const update = (i: number, patch: Partial<Row>) => setRows((r) => r.map((row, j) => (j === i ? { ...row, ...patch } : row)));

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    const directors: Director[] = [];
    let total = 0;
    for (const [i, r] of rows.entries()) {
      const pct = r.percent.trim() === "" ? 0 : Number(r.percent);
      if (!r.name.trim()) return setError(`Director ${i + 1} needs a name.`);
      if (!validSAID(r.id)) return setError(`Director ${i + 1}: that is not a valid South African ID number.`);
      if (!Number.isFinite(pct) || pct < 0 || pct > 100) return setError(`Director ${i + 1}: ownership is a percentage from 0 to 100.`);
      if (r.ubo && pct === 0) return setError(`Director ${i + 1}: a beneficial owner holds a share; enter their percentage.`);
      total += pct;
      directors.push({ legal_name: r.name.trim(), national_id: r.id.replace(/\s/g, ""), ownership_bps: Math.round(pct * 100), is_ubo: r.ubo });
    }
    if (total > 100) return setError("The directors' shares add up to more than 100%.");
    setBusy(true);
    const res = await applyBusiness({ legal_name: company.trim(), registration_number: reg.trim(), sector_code: sector, directors });
    setBusy(false);
    if (!res.ok) return setError(res.message);
    router.push("/apply/status/business");
  }

  return (
    <Shell>
      <p className={eyebrowClass}>Business account</p>
      <h1 className={titleClass}>Tell us about your company</h1>
      <p className="mt-4 max-w-lg text-lg text-[#3A3A42]">
        We verify the company with the registry, and each director verifies their identity with our verification partner.
      </p>
      <Panel className="mt-10">
        <RequireApplicant next="/apply/business">
          <form onSubmit={submit} className="space-y-6" aria-label="Business account application" noValidate>
            <Field label="Registered company name">
              <input className={inputClass} value={company} onChange={(e) => setCompany(e.target.value)} required maxLength={200} autoComplete="organization" />
            </Field>
            <Field label="Registration number" hint="As issued by CIPC, for example 2026/123456/07">
              <input className={`${inputClass} font-mono`} value={reg} onChange={(e) => setReg(e.target.value)} required maxLength={32} />
            </Field>
            <Field label="What the company does">
              <select className={inputClass} value={sector} onChange={(e) => setSector(e.target.value)}>
                {SECTORS.map((s) => (
                  <option key={s.code} value={s.code}>
                    {s.label}
                  </option>
                ))}
              </select>
            </Field>

            <fieldset className="space-y-4">
              <legend className="text-sm font-medium">Directors</legend>
              {rows.map((r, i) => (
                <div key={i} className="space-y-3 rounded-2xl bg-white p-5" aria-label={`Director ${i + 1}`} role="group">
                  <div className="flex items-center justify-between">
                    <span className={eyebrowClass}>Director {i + 1}</span>
                    {rows.length > 1 ? (
                      <button type="button" className="text-xs underline" onClick={() => setRows((rs) => rs.filter((_, j) => j !== i))}>
                        Remove
                      </button>
                    ) : null}
                  </div>
                  <Field label="Full legal name">
                    <input className={inputClass} value={r.name} onChange={(e) => update(i, { name: e.target.value })} maxLength={200} />
                  </Field>
                  <Field label="South African ID number">
                    <input className={`${inputClass} font-mono`} value={r.id} onChange={(e) => update(i, { id: e.target.value })} inputMode="numeric" maxLength={16} />
                  </Field>
                  <div className="grid grid-cols-2 gap-3">
                    <Field label="Shareholding %" hint="0 if none">
                      <input className={inputClass} value={r.percent} onChange={(e) => update(i, { percent: e.target.value })} inputMode="decimal" />
                    </Field>
                    <label className="flex items-end gap-2 pb-2.5 text-sm">
                      <input type="checkbox" checked={r.ubo} onChange={(e) => update(i, { ubo: e.target.checked })} />
                      Beneficial owner
                    </label>
                  </div>
                </div>
              ))}
              {rows.length < 20 ? (
                <button type="button" className="text-sm underline" onClick={() => setRows((rs) => [...rs, { ...empty }])}>
                  Add another director
                </button>
              ) : null}
            </fieldset>

            <label className="flex items-start gap-3 text-sm">
              <input type="checkbox" className="mt-1" checked={agreed} onChange={(e) => setAgreed(e.target.checked)} required />
              <span>
                I am authorised to apply for this company. The company agrees to the account terms and to registry checks,
                and each director will be asked to agree to identity verification.
              </span>
            </label>
            {error ? (
              <p role="alert" className="rounded-xl bg-[#F6E7E1] px-3 py-2 text-sm text-[#7A2E17]">
                {error}
              </p>
            ) : null}
            <button type="submit" className={buttonClass} disabled={busy || !agreed || !company.trim() || !reg.trim()}>
              {busy ? "Sending…" : "Continue"} <span aria-hidden="true">→</span>
            </button>
          </form>
        </RequireApplicant>
      </Panel>
    </Shell>
  );
}
