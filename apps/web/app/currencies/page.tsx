"use client";

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor } from "@/lib/money";

type Account = components["schemas"]["Account"];
type Detail = components["schemas"]["ReceivingDetail"];
type Credit = components["schemas"]["CrossBorderCredit"];
type Reason = { code: string; label: string };

const CURRENCIES = [
  { asset: "USD", name: "US dollar" },
  { asset: "EUR", name: "Euro" },
  { asset: "GBP", name: "British pound" },
] as const;

// Foreign-currency accounts: open one per currency, see how a payer pays in, and say what each payment received was
// for. The reason goes with the payment to the Reserve Bank's cross-border reporting.
export default function CurrenciesPage() {
  const router = useRouter();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [details, setDetails] = useState<Detail[]>([]);
  const [credits, setCredits] = useState<Credit[]>([]);
  const [reasons, setReasons] = useState<Reason[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);
  const [busy, setBusy] = useState("");

  const load = useCallback(async () => {
    const [acc, det, cr, rs] = await Promise.all([
      api.GET("/v1/accounts"),
      api.GET("/v1/receiving-details"),
      api.GET("/v1/cross-border-credits"),
      api.GET("/v1/payment-reasons"),
    ]);
    if (acc.response.status === 401) return router.push("/sign-in");
    setAccounts(acc.data?.accounts ?? []);
    setDetails(det.data?.details ?? []);
    setCredits(cr.data?.credits ?? []);
    setReasons(rs.data?.reasons ?? []);
  }, [router]);

  useEffect(() => {
    void load();
  }, [load]);

  async function open(asset: string) {
    setError(null);
    setDone(null);
    setBusy(asset);
    const { error, response } = await api.POST("/v1/accounts", { body: { asset: asset as "USD" } });
    setBusy("");
    if (!response.ok) return setError(errorText(error, `Request failed (${response.status}).`));
    setDone(`Your ${asset} account is open.`);
    await load();
  }

  async function classify(credit: string, code: string, note: string) {
    setError(null);
    setDone(null);
    const { error, response } = await api.POST("/v1/cross-border-credits/{credit}/reason", {
      params: { path: { credit } },
      body: { reason_code: code, note },
    });
    if (!response.ok) return setError(errorText(error, `Request failed (${response.status}).`));
    setDone("Thank you. The reason is recorded with the payment.");
    await load();
  }

  const awaiting = credits.filter((c) => c.status === "AWAITING_REASON");
  return (
    <section className="space-y-10" aria-labelledby="currencies-heading">
      <div>
        <h1 id="currencies-heading" className="text-3xl font-semibold tracking-tight">
          Currencies
        </h1>
        <p className="mt-2 max-w-2xl text-stone-600">
          Hold dollars, euros and pounds, and get paid by customers abroad. Foreign balances are held at our partner bank
          and are not covered by deposit insurance.
        </p>
      </div>

      {error ? <Notice tone="error">{error}</Notice> : null}
      {done ? <Notice tone="success">{done}</Notice> : null}

      {awaiting.length ? (
        <div className="rounded-2xl border border-sky-200 bg-sky-50 p-8" aria-labelledby="reasons-heading">
          <h2 id="reasons-heading" className="text-2xl font-light tracking-tight">
            Tell us what {awaiting.length === 1 ? "this payment was" : "these payments were"} for
          </h2>
          <p className="mt-2 text-sm text-stone-600">
            The Reserve Bank requires a reason for every payment received from abroad.
          </p>
          <ul className="mt-6 space-y-4">
            {awaiting.map((c) => (
              <ReasonForm key={c.credit_id} credit={c} reasons={reasons} onSave={classify} />
            ))}
          </ul>
        </div>
      ) : null}

      <ul className="grid gap-4 sm:grid-cols-3">
        {CURRENCIES.map((c) => {
          const account = accounts.find((a) => a.asset === c.asset);
          const detail = details.find((d) => d.asset === c.asset);
          return (
            <li key={c.asset} className="flex flex-col justify-between gap-6 rounded-2xl border border-stone-200 bg-white shadow-sm p-8">
              <div>
                <p className="text-sm text-stone-500">{c.name}</p>
                <p className="mt-1 text-3xl font-light tabular-nums">
                  {account ? formatMinor(account.available_minor, c.asset) : c.asset}
                </p>
                {detail ? (
                  <p className="mt-3 text-sm text-stone-600">
                    Payers quote <span className="font-mono text-stone-900">{detail.reference}</span>
                  </p>
                ) : null}
              </div>
              {account ? (
                <span className="text-sm text-stone-500">Open</span>
              ) : (
                <button
                  type="button"
                  disabled={busy !== ""}
                  onClick={() => open(c.asset)}
                  className="self-start rounded-full bg-stone-900 px-6 py-3 text-base text-white hover:bg-black disabled:opacity-50"
                >
                  {busy === c.asset ? "Opening…" : `Open ${c.asset} account`}
                </button>
              )}
            </li>
          );
        })}
      </ul>

      {credits.length ? (
        <div aria-labelledby="received-heading">
          <h2 id="received-heading" className="text-2xl font-light tracking-tight">
            Received from abroad
          </h2>
          <ul className="mt-4 divide-y divide-stone-200">
            {credits.map((c) => (
              <li key={c.credit_id} className="flex flex-wrap justify-between gap-2 py-3 text-sm">
                <span className="tabular-nums">{formatMinor(c.amount_minor, c.asset)}</span>
                <span className="text-stone-500">{c.booked_at.slice(0, 10)}</span>
                <span>{c.status === "CLASSIFIED" ? reasons.find((r) => r.code === c.reason_code)?.label ?? c.reason_code : "Reason needed"}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}

function ReasonForm({ credit, reasons, onSave }: { credit: Credit; reasons: Reason[]; onSave: (id: string, code: string, note: string) => Promise<void> }) {
  const [code, setCode] = useState("");
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);
  return (
    <li className="rounded-2xl bg-white p-5" aria-label={`Payment of ${formatMinor(credit.amount_minor, credit.asset)}`}>
      <p className="font-medium tabular-nums">
        {formatMinor(credit.amount_minor, credit.asset)} <span className="font-normal text-stone-500">on {credit.booked_at.slice(0, 10)}</span>
      </p>
      <form
        className="mt-3 grid gap-3 sm:grid-cols-[1fr_1fr_auto]"
        onSubmit={async (e) => {
          e.preventDefault();
          setSaving(true);
          await onSave(credit.credit_id, code, note);
          setSaving(false);
        }}
      >
        <label className="text-sm">
          <span className="sr-only">Reason</span>
          <select value={code} onChange={(e) => setCode(e.target.value)} required className="w-full rounded-xl border border-stone-300 bg-white px-3 py-2">
            <option value="">Choose a reason</option>
            {reasons.map((r) => (
              <option key={r.code} value={r.code}>
                {r.label}
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm">
          <span className="sr-only">Note</span>
          <input
            value={note}
            onChange={(e) => setNote(e.target.value)}
            maxLength={200}
            placeholder="Invoice number or a short description"
            className="w-full rounded-xl border border-stone-300 px-3 py-2"
          />
        </label>
        <button type="submit" disabled={!code || saving} className="rounded-full bg-stone-900 px-6 py-2 text-white disabled:opacity-50">
          {saving ? "Saving…" : "Save"}
        </button>
      </form>
    </li>
  );
}
