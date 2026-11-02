"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { parseToMinor } from "@/lib/money";

type Account = components["schemas"]["Account"];

type Row = {
  key: string;
  amount: string;
  beneficiary: string;
  accountNumber: string;
  bankCode: string;
  reference: string;
};

const blankRow = (n: number): Row => ({
  key: `p${n}`,
  amount: "",
  beneficiary: "",
  accountNumber: "",
  bankCode: "",
  reference: "",
});

export default function NewPayoutPage() {
  const router = useRouter();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountId, setAccountId] = useState("");
  const [rows, setRows] = useState<Row[]>([blankRow(1)]);
  // One key per batch. Retrying the same submission cannot create a second batch.
  const [idemKey, setIdemKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setIdemKey(crypto.randomUUID());
    api.GET("/v1/accounts").then(({ data, response }) => {
      if (response.status === 401) return router.push("/sign-in");
      const zar = (data?.accounts ?? []).filter((a) => a.asset === "ZAR");
      setAccounts(zar);
      if (zar[0]) setAccountId(zar[0].account_id);
    });
  }, [router]);

  function update(index: number, patch: Partial<Row>) {
    setRows((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch } : r)));
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    if (!idemKey) return;
    const items = [];
    for (const r of rows) {
      const minor = parseToMinor(r.amount);
      if (!minor) return setError(`Payment ${r.key}: enter an amount greater than zero.`);
      items.push({
        item_key: r.key,
        amount_minor: minor,
        beneficiary_name: r.beneficiary,
        account_number: r.accountNumber,
        bank_code: r.bankCode,
        reference: r.reference,
      });
    }
    setBusy(true);
    const { data, error, response } = await api.POST("/v1/payout-batches", {
      params: { header: { "Idempotency-Key": idemKey } },
      body: { from_account_id: accountId, asset: "ZAR", items },
    });
    setBusy(false);
    if (response.status === 401) return router.push("/sign-in");
    if (error || !data) return setError(errorText(error, "The batch was not created. Try again."));
    router.push(`/payouts/${encodeURIComponent(data.batch_id)}`);
  }

  return (
    <section className="space-y-6">
      <h1 className="text-2xl font-semibold">New bulk payout</h1>
      <Notice tone="info">
        You create the batch. A different person must approve it before any payment is sent.
      </Notice>
      {error ? <Notice tone="error">{error}</Notice> : null}
      <form onSubmit={submit} className="space-y-6" aria-label="New bulk payout">
        <label className="block max-w-sm text-sm">
          <span className="font-medium">Pay from</span>
          <select
            value={accountId}
            onChange={(e) => setAccountId(e.target.value)}
            required
            className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2 text-sm"
          >
            {accounts.map((a) => (
              <option key={a.account_id} value={a.account_id}>
                ZAR account
              </option>
            ))}
          </select>
        </label>

        <div className="space-y-4">
          {rows.map((r, i) => (
            <fieldset key={r.key} className="grid gap-3 rounded-lg border border-stone-200 bg-white p-4 sm:grid-cols-2">
              <legend className="px-1 text-sm font-medium">Payment {i + 1}</legend>
              <label className="text-sm">
                <span className="text-stone-600">Amount (R)</span>
                <input
                  aria-label={`Amount ${i + 1}`}
                  value={r.amount}
                  onChange={(e) => update(i, { amount: e.target.value })}
                  inputMode="decimal"
                  required
                  className="mt-1 w-full rounded-md border border-stone-300 px-3 py-2 text-sm tabular-nums"
                />
              </label>
              <label className="text-sm">
                <span className="text-stone-600">Beneficiary</span>
                <input
                  aria-label={`Beneficiary ${i + 1}`}
                  value={r.beneficiary}
                  onChange={(e) => update(i, { beneficiary: e.target.value })}
                  required
                  maxLength={140}
                  className="mt-1 w-full rounded-md border border-stone-300 px-3 py-2 text-sm"
                />
              </label>
              <label className="text-sm">
                <span className="text-stone-600">Account number</span>
                <input
                  aria-label={`Account number ${i + 1}`}
                  value={r.accountNumber}
                  onChange={(e) => update(i, { accountNumber: e.target.value })}
                  required
                  className="mt-1 w-full rounded-md border border-stone-300 px-3 py-2 text-sm"
                />
              </label>
              <label className="text-sm">
                <span className="text-stone-600">Bank code</span>
                <input
                  aria-label={`Bank code ${i + 1}`}
                  value={r.bankCode}
                  onChange={(e) => update(i, { bankCode: e.target.value })}
                  required
                  className="mt-1 w-full rounded-md border border-stone-300 px-3 py-2 text-sm"
                />
              </label>
              <label className="text-sm sm:col-span-2">
                <span className="text-stone-600">Reference</span>
                <input
                  aria-label={`Reference ${i + 1}`}
                  value={r.reference}
                  onChange={(e) => update(i, { reference: e.target.value })}
                  required
                  maxLength={140}
                  className="mt-1 w-full rounded-md border border-stone-300 px-3 py-2 text-sm"
                />
              </label>
              {rows.length > 1 ? (
                <button
                  type="button"
                  onClick={() => setRows((prev) => prev.filter((_, j) => j !== i))}
                  className="justify-self-start text-sm text-red-700 underline sm:col-span-2"
                >
                  Remove this payment
                </button>
              ) : null}
            </fieldset>
          ))}
        </div>

        <div className="flex flex-wrap gap-3">
          <button
            type="button"
            onClick={() => setRows((prev) => [...prev, blankRow(prev.length + 1)])}
            disabled={rows.length >= 500}
            className="rounded-md border border-stone-300 bg-white px-4 py-2 text-sm hover:bg-stone-100 disabled:opacity-50"
          >
            Add a payment
          </button>
          <button
            type="submit"
            disabled={busy || !accountId}
            className="rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white hover:bg-stone-800 disabled:opacity-50"
          >
            {busy ? "Creating…" : "Create batch"}
          </button>
        </div>
      </form>
    </section>
  );
}
