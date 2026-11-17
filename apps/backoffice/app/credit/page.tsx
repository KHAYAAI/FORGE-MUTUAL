"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { formatMinor, parseToMinor } from "@/lib/money";
import { hasRole, useStaff } from "@/lib/useStaff";

type Facility = {
  facility_id: string;
  product_id: string;
  status: string;
  requested_minor: string;
  rate_bps: number;
  term_months: number;
  fee_bps: number;
  total_repayable_minor: string;
  applied_at: string;
  purpose: string;
  party_id: string;
  assessment: { inflow_90d_minor: string; monthly_inflow_minor: string; suggested_max_minor: string; notes: string[] };
};

const NAMES: Record<string, string> = { overdraft: "Overdraft", "term-loan": "Working-capital loan", "revenue-based": "Revenue-based financing" };
const MIN_NOTE = 20;

// The credit desk: applications waiting for a decision, each with the assessment made from the customer's account.
export default function CreditPage() {
  const router = useRouter();
  const staff = useStaff("/credit");
  const [apps, setApps] = useState<Facility[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  async function load() {
    const res = await fetch("/api/v1/credit/applications", { cache: "no-store" });
    if (res.status === 401) return router.push("/sign-in");
    if (!res.ok) return setError(res.status === 403 ? "You need the credit role to see the credit desk." : "The applications could not be loaded.");
    const body = (await res.json()) as { facilities?: Facility[] };
    setError(null);
    setApps(body.facilities ?? []);
  }

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router]);

  async function decide(id: string, approve: boolean, amount: string, note: string) {
    setNotice(null);
    setError(null);
    const approved = amount.trim() ? parseToMinor(amount) : "";
    if (approved === null) return setError("Enter the approved amount in rand, or leave it empty for the amount asked.");
    const res = await fetch(`/api/v1/credit/applications/${encodeURIComponent(id)}/decision`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ approve, approved_minor: approved, note }),
    });
    const body = (await res.json().catch(() => null)) as { error?: { message?: string } } | null;
    if (!res.ok) return setError(body?.error?.message ?? "The decision was not recorded.");
    setNotice(approve ? "Approved. The customer can now accept the offer." : "Declined.");
    await load();
  }

  const canDecide = hasRole(staff ?? null, "credit");
  return (
    <section aria-labelledby="credit-heading" className="space-y-6">
      <h1 id="credit-heading" className="text-2xl font-semibold">
        Credit applications
      </h1>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {notice ? <Notice tone="success">{notice}</Notice> : null}
      {apps && apps.length === 0 ? <p className="text-stone-600">No applications are waiting for a decision.</p> : null}
      <ul className="space-y-4">
        {(apps ?? []).map((f) => (
          <Application key={f.facility_id} f={f} canDecide={canDecide} onDecide={decide} />
        ))}
      </ul>
    </section>
  );
}

function Application({ f, canDecide, onDecide }: { f: Facility; canDecide: boolean; onDecide: (id: string, approve: boolean, amount: string, note: string) => Promise<void> }) {
  const [amount, setAmount] = useState("");
  const [note, setNote] = useState("");
  const over = BigInt(f.requested_minor) > BigInt(f.assessment.suggested_max_minor || "0");
  return (
    <li className="rounded-lg border border-stone-200 bg-white p-5" aria-label={`Application ${f.facility_id.slice(0, 8)}`}>
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="text-lg font-semibold">
          {NAMES[f.product_id] ?? f.product_id}: {formatMinor(f.requested_minor, "ZAR")}
          {f.term_months ? ` over ${f.term_months} months` : ""}
        </h2>
        <span className="text-sm text-stone-500">Applied {f.applied_at.slice(0, 10)}</span>
      </div>
      <p className="mt-1 text-sm text-stone-600">Party {f.party_id}. Purpose: {f.purpose || "not given"}.</p>
      <dl className="mt-3 grid grid-cols-2 gap-1 text-sm sm:grid-cols-4">
        <dt className="text-stone-500">Money in, 90 days</dt>
        <dd>{formatMinor(f.assessment.inflow_90d_minor, "ZAR")}</dd>
        <dt className="text-stone-500">Policy allows up to</dt>
        <dd className={over ? "font-semibold text-red-700" : ""}>{formatMinor(f.assessment.suggested_max_minor, "ZAR")}</dd>
        <dt className="text-stone-500">Total repayable</dt>
        <dd>{formatMinor(f.total_repayable_minor, "ZAR")}</dd>
        <dt className="text-stone-500">Rate / fee</dt>
        <dd>{f.fee_bps ? `${f.fee_bps / 100}% fee` : `${(f.rate_bps / 100).toFixed(2)}% a year`}</dd>
      </dl>
      {f.assessment.notes?.length ? (
        <ul className="mt-2 list-disc pl-5 text-sm text-red-700">
          {f.assessment.notes.map((n) => (
            <li key={n}>{n}</li>
          ))}
        </ul>
      ) : null}
      {canDecide ? (
        <form className="mt-4 space-y-3" onSubmit={(e) => e.preventDefault()}>
          <label className="block text-sm">
            <span className="font-medium">Approved amount (R), if less than asked</span>
            <input value={amount} onChange={(e) => setAmount(e.target.value)} inputMode="decimal" className="mt-1 w-48 rounded-md border border-stone-300 px-3 py-2" />
          </label>
          <label className="block text-sm">
            <span className="font-medium">Your reasons (at least {MIN_NOTE} characters)</span>
            <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} className="mt-1 w-full rounded-md border border-stone-300 px-3 py-2" />
          </label>
          <div className="flex gap-2">
            <button type="button" disabled={note.trim().length < MIN_NOTE} onClick={() => onDecide(f.facility_id, true, amount, note)} className="rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-50">
              Approve
            </button>
            <button type="button" disabled={note.trim().length < MIN_NOTE} onClick={() => onDecide(f.facility_id, false, "", note)} className="rounded-md border border-stone-400 px-4 py-2 text-sm font-medium disabled:opacity-50">
              Decline
            </button>
          </div>
        </form>
      ) : null}
    </li>
  );
}
