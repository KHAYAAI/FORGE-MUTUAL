"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useCallback, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor, parseToMinor } from "@/lib/money";

type Link = components["schemas"]["PaymentLink"];
type Settlement = components["schemas"]["Settlement"];
type Account = components["schemas"]["Account"];

const STATUS: Record<string, string> = { OPEN: "Waiting for payment", PAID: "Paid", EXPIRED: "Expired", CANCELLED: "Cancelled" };
const key = () => crypto.randomUUID();

// Card payments for a business: a payment link to send, or a point-of-sale request to show at the counter. Takings
// settle to the current account the next day, less the merchant discount.
export default function GetPaidPage() {
  const router = useRouter();
  const [links, setLinks] = useState<Link[]>([]);
  const [settlements, setSettlements] = useState<Settlement[]>([]);
  const [mdr, setMdr] = useState(0);
  const [account, setAccount] = useState<Account | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);
  const [form, setForm] = useState({ merchant: "", amount: "", description: "", reference: "", channel: "link" as "link" | "pos" });

  const load = useCallback(async () => {
    const [l, s, a] = await Promise.all([api.GET("/v1/payment-links"), api.GET("/v1/settlements"), api.GET("/v1/accounts")]);
    if (a.response.status === 401) return router.push("/sign-in");
    setLinks(l.data?.links ?? []);
    setSettlements(s.data?.settlements ?? []);
    setMdr(s.data?.mdr_bps ?? 0);
    setAccount((a.data?.accounts ?? []).find((x) => x.asset === "ZAR") ?? null);
  }, [router]);

  useEffect(() => {
    void load();
  }, [load]);

  const payUrl = (l: Link) => `${window.location.origin}/pay/${l.token}`;

  async function create(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setDone(null);
    const minor = parseToMinor(form.amount);
    if (!minor) return setError("Enter an amount greater than zero, with at most two decimal places.");
    if (!account) return setError("You need a rand current account for card takings to settle to.");
    const res = await api.POST("/v1/payment-links", {
      params: { header: { "Idempotency-Key": key() } },
      body: { settlement_account_id: account.account_id, amount_minor: minor, description: form.description, reference: form.reference, channel: form.channel, merchant_name: form.merchant },
    });
    if (!res.response.ok) return setError(errorText(res.error, `Request failed (${res.response.status}).`));
    setDone(form.channel === "pos" ? "Show the customer the payment page, or let them scan its address." : "Your payment link is ready. Copy it and send it to your customer.");
    setForm({ ...form, amount: "", description: "", reference: "" });
    await load();
  }

  async function cancel(id: string) {
    const res = await api.POST("/v1/payment-links/{link}/cancel", { params: { path: { link: id } } });
    if (!res.response.ok) return setError(errorText(res.error, "The link was not cancelled."));
    await load();
  }

  async function refund(l: Link) {
    const left = (BigInt(l.amount_minor) - BigInt(l.refunded_minor)).toString();
    const res = await api.POST("/v1/payment-links/{link}/refund", {
      params: { path: { link: l.link_id }, header: { "Idempotency-Key": key() } },
      body: { amount_minor: left },
    });
    if (!res.response.ok) return setError(errorText(res.error, "The refund was not made."));
    setDone("Refunded to the customer's card.");
    await load();
  }

  return (
    <section className="space-y-10" aria-labelledby="get-paid-heading">
      <div>
        <h1 id="get-paid-heading" className="text-3xl font-semibold tracking-tight">
          Get paid
        </h1>
        <p className="mt-2 max-w-2xl text-stone-600">
          Take card payments with a link or at the counter. Takings arrive in your account the next day
          {mdr ? `, less a ${(mdr / 100).toFixed(2)}% card fee` : ""}.
        </p>
      </div>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {done ? <Notice tone="success">{done}</Notice> : null}

      <form onSubmit={create} className="grid max-w-3xl gap-4 rounded-2xl border border-stone-200 bg-white shadow-sm p-8 sm:grid-cols-2" aria-label="New payment request">
        <label className="text-sm">
          <span className="font-medium">Your business name, as the customer sees it</span>
          <input required value={form.merchant} onChange={(e) => setForm({ ...form, merchant: e.target.value })} maxLength={80} className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2" />
        </label>
        <label className="text-sm">
          <span className="font-medium">Amount (R)</span>
          <input required value={form.amount} onChange={(e) => setForm({ ...form, amount: e.target.value })} inputMode="decimal" className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2" />
        </label>
        <label className="text-sm">
          <span className="font-medium">What it is for</span>
          <input required value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} maxLength={140} className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2" />
        </label>
        <label className="text-sm">
          <span className="font-medium">Your reference (optional)</span>
          <input value={form.reference} onChange={(e) => setForm({ ...form, reference: e.target.value })} maxLength={35} className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2" />
        </label>
        <fieldset className="text-sm sm:col-span-2">
          <legend className="font-medium">How the customer pays</legend>
          <label className="mr-6 inline-flex items-center gap-2">
            <input type="radio" name="channel" checked={form.channel === "link"} onChange={() => setForm({ ...form, channel: "link" })} /> Send a link (valid a week)
          </label>
          <label className="inline-flex items-center gap-2">
            <input type="radio" name="channel" checked={form.channel === "pos"} onChange={() => setForm({ ...form, channel: "pos" })} /> At the counter (valid an hour)
          </label>
        </fieldset>
        <div>
          <button type="submit" className="rounded-full bg-stone-900 px-6 py-3 text-white hover:bg-black">
            Create
          </button>
        </div>
      </form>

      {links.length ? (
        <div>
          <h2 className="text-2xl font-light tracking-tight">Payment requests</h2>
          <ul className="mt-4 divide-y divide-stone-200">
            {links.map((l) => (
              <li key={l.link_id} className="flex flex-wrap items-center justify-between gap-3 py-4 text-sm" aria-label={`${l.description}, ${formatMinor(l.amount_minor, "ZAR")}`}>
                <div>
                  <p className="font-medium">
                    {l.description} <span className="tabular-nums">{formatMinor(l.amount_minor, "ZAR")}</span>
                  </p>
                  <p className="text-stone-500">
                    {STATUS[l.status]}
                    {l.refunded_minor !== "0" ? `, ${formatMinor(l.refunded_minor, "ZAR")} refunded` : ""}
                  </p>
                </div>
                <div className="flex gap-2">
                  {l.status === "OPEN" ? (
                    <>
                      <a href={`/pay/${l.token}`} className="rounded-full border border-stone-900 px-4 py-2" target="_blank" rel="noopener noreferrer">
                        Open payment page
                      </a>
                      <button type="button" className="rounded-full border border-stone-900 px-4 py-2" onClick={() => void navigator.clipboard?.writeText(payUrl(l))}>
                        Copy link
                      </button>
                      <button type="button" className="underline" onClick={() => cancel(l.link_id)}>
                        Cancel
                      </button>
                    </>
                  ) : null}
                  {l.status === "PAID" && l.refunded_minor !== l.amount_minor ? (
                    <button type="button" className="underline" onClick={() => refund(l)}>
                      Refund
                    </button>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      {settlements.length ? (
        <div>
          <h2 className="text-2xl font-light tracking-tight">Settlements</h2>
          <table className="mt-4 w-full text-left text-sm">
            <thead className="text-stone-500">
              <tr>
                <th className="py-2 font-normal">Day</th>
                <th className="font-normal">Payments</th>
                <th className="font-normal">Takings</th>
                <th className="font-normal">Card fee</th>
                <th className="font-normal">Paid to you</th>
              </tr>
            </thead>
            <tbody className="tabular-nums">
              {settlements.map((s) => (
                <tr key={s.settlement_id} className="border-t border-stone-200">
                  <td className="py-2">{s.day}</td>
                  <td>{s.payments}</td>
                  <td>{formatMinor(s.gross_minor, "ZAR")}</td>
                  <td>{formatMinor(s.fee_minor, "ZAR")}</td>
                  <td>{formatMinor(s.net_minor, "ZAR")}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </section>
  );
}
