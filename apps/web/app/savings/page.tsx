"use client";

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor, parseToMinor } from "@/lib/money";

type Product = components["schemas"]["DepositProduct"];
type DepositAccount = components["schemas"]["DepositAccount"];
type Account = components["schemas"]["Account"];

const pct = (bps: number) => `${(bps / 100).toFixed(2)}%`;
const newKey = () => crypto.randomUUID();

// Savings and notice deposits: open an account, move money in and out, and watch interest accrue.
export default function SavingsPage() {
  const router = useRouter();
  const [products, setProducts] = useState<Product[]>([]);
  const [accounts, setAccounts] = useState<DepositAccount[]>([]);
  const [current, setCurrent] = useState<Account | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);

  const load = useCallback(async () => {
    const [p, d, a] = await Promise.all([api.GET("/v1/deposit-products"), api.GET("/v1/deposit-accounts"), api.GET("/v1/accounts")]);
    if (a.response.status === 401) return router.push("/sign-in");
    setProducts(p.data?.products ?? []);
    setAccounts(d.data?.accounts ?? []);
    const owned = new Set((d.data?.accounts ?? []).map((x) => x.account_id));
    setCurrent((a.data?.accounts ?? []).find((x) => x.asset === "ZAR" && !owned.has(x.account_id)) ?? null);
  }, [router]);

  useEffect(() => {
    void load();
  }, [load]);

  async function openProduct(id: string) {
    setError(null);
    setDone(null);
    const { error, response } = await api.POST("/v1/deposit-accounts", { body: { product_id: id } });
    if (!response.ok) return setError(errorText(error, `Request failed (${response.status}).`));
    setDone("Your account is open.");
    await load();
  }

  async function move(kind: "in" | "out", acc: DepositAccount, amount: string) {
    setError(null);
    setDone(null);
    const minor = parseToMinor(amount);
    if (!minor) return setError("Enter an amount greater than zero, with at most two decimal places.");
    if (!current) return setError("You need a rand current account to move money.");
    const body = kind === "in"
      ? { from_account_id: current.account_id, to_account_id: acc.account_id, amount_minor: minor }
      : { from_account_id: acc.account_id, to_account_id: current.account_id, amount_minor: minor };
    const params = { header: { "Idempotency-Key": newKey() } };
    const res = kind === "in"
      ? await api.POST("/v1/deposits", { params, body })
      : await api.POST("/v1/withdrawals", { params, body });
    if (!res.response.ok) return setError(errorText(res.error, `Request failed (${res.response.status}).`));
    const state = (res.data as { state?: string; notice?: { due_on: string } } | undefined);
    setDone(state?.state === "NOTICE_GIVEN" ? `Notice given. The money will be paid on ${state.notice?.due_on}.` : "Done.");
    await load();
  }

  async function cancel(notice: string) {
    const { error, response } = await api.POST("/v1/notices/{notice}/cancel", { params: { path: { notice } } });
    if (!response.ok) return setError(errorText(error, `Request failed (${response.status}).`));
    setDone("Notice cancelled.");
    await load();
  }

  const unopened = products.filter((p) => !accounts.some((a) => a.product_id === p.product_id));
  return (
    <section className="space-y-10" aria-labelledby="savings-heading">
      <div>
        <h1 id="savings-heading" className="text-3xl font-semibold tracking-tight">
          Savings
        </h1>
        <p className="mt-2 max-w-2xl text-stone-600">
          Interest is worked out every day on your balance and paid into your account on the first of each month.
        </p>
      </div>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {done ? <Notice tone="success">{done}</Notice> : null}

      <ul className="grid gap-4 sm:grid-cols-2">
        {accounts.map((a) => {
          const p = products.find((x) => x.product_id === a.product_id);
          return <SavingsCard key={a.account_id} account={a} name={p?.name ?? a.product_id} onMove={move} onCancel={cancel} />;
        })}
      </ul>

      {unopened.length ? (
        <div>
          <h2 className="text-2xl font-light tracking-tight">Open an account</h2>
          <ul className="mt-4 grid gap-4 sm:grid-cols-3">
            {unopened.map((p) => (
              <li key={p.product_id} className="flex flex-col justify-between gap-6 rounded-2xl border border-stone-200 bg-white p-5 shadow-sm sm:p-8">
                <div>
                  <p className="text-xl font-light">{p.name}</p>
                  <p className="mt-1 text-3xl font-light">{pct(p.rate_bps)}</p>
                  <p className="mt-2 text-sm text-stone-600">
                    {p.notice_days ? `Give ${p.notice_days} days' notice to withdraw.` : "Withdraw any time."}
                  </p>
                </div>
                {p.available ? (
                  <button type="button" onClick={() => openProduct(p.product_id)} className="self-start rounded-full bg-stone-900 px-6 py-3 text-white hover:bg-black">
                    Open {p.name}
                  </button>
                ) : (
                  <p className="text-sm text-stone-500">Not yet available</p>
                )}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}

function SavingsCard({ account, name, onMove, onCancel }: {
  account: DepositAccount;
  name: string;
  onMove: (kind: "in" | "out", a: DepositAccount, amount: string) => Promise<void>;
  onCancel: (notice: string) => Promise<void>;
}) {
  const [amount, setAmount] = useState("");
  const pending = account.notices.filter((n) => n.status === "PENDING");
  return (
    <li className="rounded-2xl border border-stone-200 bg-white p-5 shadow-sm sm:p-8" aria-label={name}>
      <div className="flex items-baseline justify-between">
        <p className="text-sm text-stone-500">{name}</p>
        <p className="text-sm text-stone-500">{pct(account.rate_bps)} a year</p>
      </div>
      <p className="mt-1 text-3xl font-semibold tracking-tight tabular-nums" data-testid="savings-balance">{formatMinor(account.balance_minor, "ZAR")}</p>
      <dl className="mt-3 grid grid-cols-2 gap-1 text-sm">
        <dt className="text-stone-500">Interest this month</dt>
        <dd className="tabular-nums">{formatMinor(account.accrued_minor, "ZAR")}</dd>
        <dt className="text-stone-500">Interest paid so far</dt>
        <dd className="tabular-nums">{formatMinor(account.interest_paid_minor, "ZAR")}</dd>
        {account.notice_days ? (
          <>
            <dt className="text-stone-500">Under notice</dt>
            <dd className="tabular-nums">{formatMinor(account.under_notice_minor, "ZAR")}</dd>
          </>
        ) : null}
      </dl>
      <form className="mt-6 flex flex-wrap items-end gap-2" onSubmit={(e) => e.preventDefault()}>
        <label className="text-sm">
          <span className="block font-medium">Amount (R)</span>
          <input value={amount} onChange={(e) => setAmount(e.target.value)} inputMode="decimal" className="mt-1 w-36 rounded-xl border border-stone-300 bg-white px-3 py-2" />
        </label>
        <button type="button" onClick={() => onMove("in", account, amount)} className="rounded-full bg-stone-900 px-5 py-2 text-white hover:bg-black">
          Add money
        </button>
        <button type="button" onClick={() => onMove("out", account, amount)} className="rounded-full border border-stone-900 px-5 py-2 hover:bg-white">
          {account.notice_days ? "Give notice" : "Withdraw"}
        </button>
      </form>
      {pending.length ? (
        <ul className="mt-4 space-y-2 text-sm">
          {pending.map((n) => (
            <li key={n.notice_id} className="flex items-center justify-between rounded-xl bg-white px-4 py-2">
              <span>
                {formatMinor(n.amount_minor, "ZAR")} paid on {n.due_on}
              </span>
              <button type="button" className="underline" onClick={() => onCancel(n.notice_id)}>
                Cancel
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </li>
  );
}
