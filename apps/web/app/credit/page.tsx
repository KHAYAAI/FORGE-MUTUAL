"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useCallback, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor, parseToMinor } from "@/lib/money";

type Product = components["schemas"]["LoanProduct"];
type Facility = components["schemas"]["Facility"];
type Account = components["schemas"]["Account"];

const STATUS: Record<string, string> = {
  ASSESSED: "Being reviewed",
  APPROVED: "Offer ready",
  DECLINED: "Not approved",
  ACTIVE: "Active",
  SETTLED: "Repaid",
  CANCELLED: "Cancelled",
};
const key = () => crypto.randomUUID();
const terms = (p: Product) =>
  p.fee_bps ? `${p.fee_bps / 100}% fixed fee, repaid with ${p.sweep_bps / 100}% of money you receive` : `${(p.rate_bps / 100).toFixed(2)}% a year`;

// Credit: apply, accept an offer on its full cost, and use or repay a facility.
export default function CreditPage() {
  const router = useRouter();
  const [products, setProducts] = useState<Product[]>([]);
  const [facilities, setFacilities] = useState<Facility[]>([]);
  const [current, setCurrent] = useState<Account | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);
  const [product, setProduct] = useState("overdraft");
  const [amount, setAmount] = useState("");
  const [term, setTerm] = useState("12");
  const [purpose, setPurpose] = useState("");

  const load = useCallback(async () => {
    const [p, f, a] = await Promise.all([api.GET("/v1/loan-products"), api.GET("/v1/facilities"), api.GET("/v1/accounts")]);
    if (a.response.status === 401) return router.push("/sign-in");
    setProducts(p.data?.products ?? []);
    setFacilities(f.data?.facilities ?? []);
    setCurrent((a.data?.accounts ?? []).find((x) => x.asset === "ZAR" && (x.product_id ?? "").includes("current")) ?? (a.data?.accounts ?? []).find((x) => x.asset === "ZAR") ?? null);
  }, [router]);

  useEffect(() => {
    void load();
  }, [load]);

  const fail = (e: unknown, status: number) => setError(errorText(e, `Request failed (${status}).`));

  async function apply(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setDone(null);
    const minor = parseToMinor(amount);
    if (!minor) return setError("Enter an amount greater than zero, with at most two decimal places.");
    if (!current) return setError("You need a rand current account to borrow.");
    const res = await api.POST("/v1/facilities", {
      params: { header: { "Idempotency-Key": key() } },
      body: { product_id: product, amount_minor: minor, term_months: product === "term-loan" ? Number(term) : 0, current_account_id: current.account_id, purpose },
    });
    if (!res.response.ok) return fail(res.error, res.response.status);
    setDone("Thank you. A credit officer will review your application.");
    setAmount("");
    await load();
  }

  async function accept(id: string) {
    const res = await api.POST("/v1/facilities/{facility}/accept", { params: { path: { facility: id } }, body: { accepts_terms: true } });
    if (!res.response.ok) return fail(res.error, res.response.status);
    setDone("Accepted. The facility is active.");
    await load();
  }

  async function move(id: string, kind: "draw" | "repay", value: string) {
    setError(null);
    setDone(null);
    const minor = parseToMinor(value);
    if (!minor) return setError("Enter an amount greater than zero.");
    const opts = { params: { path: { facility: id }, header: { "Idempotency-Key": key() } }, body: { amount_minor: minor } };
    const res = kind === "draw" ? await api.POST("/v1/facilities/{facility}/draw", opts) : await api.POST("/v1/facilities/{facility}/repay", opts);
    if (!res.response.ok) return fail(res.error, res.response.status);
    setDone(kind === "draw" ? "The money is in your current account." : "Thank you. Your repayment is recorded.");
    await load();
  }

  const name = (id: string) => products.find((p) => p.product_id === id)?.name ?? id;
  const available = products.filter((p) => p.available);
  return (
    <section className="space-y-10" aria-labelledby="credit-heading">
      <div>
        <h1 id="credit-heading" className="text-4xl font-light tracking-tight">
          Credit
        </h1>
        <p className="mt-2 max-w-2xl text-stone-600">
          Credit for your business, assessed on the money that moves through your account. Every offer shows its full cost
          before you accept it.
        </p>
      </div>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {done ? <Notice tone="success">{done}</Notice> : null}

      {facilities.length ? (
        <ul className="grid gap-4 sm:grid-cols-2">
          {facilities.map((f) => (
            <FacilityCard key={f.facility_id} f={f} name={name(f.product_id)} onAccept={accept} onMove={move} />
          ))}
        </ul>
      ) : null}

      {available.length ? (
        <form onSubmit={apply} className="max-w-xl space-y-4 rounded-[2rem] bg-stone-100 p-8" aria-label="Apply for credit">
          <h2 className="text-2xl font-light tracking-tight">Apply</h2>
          <label className="block text-sm">
            <span className="font-medium">Product</span>
            <select value={product} onChange={(e) => setProduct(e.target.value)} className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2">
              {available.map((p) => (
                <option key={p.product_id} value={p.product_id}>
                  {p.name}: {terms(p)}
                </option>
              ))}
            </select>
          </label>
          <label className="block text-sm">
            <span className="font-medium">Amount (R)</span>
            <input value={amount} onChange={(e) => setAmount(e.target.value)} inputMode="decimal" className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2" />
          </label>
          {product === "term-loan" ? (
            <label className="block text-sm">
              <span className="font-medium">Term (months)</span>
              <input value={term} onChange={(e) => setTerm(e.target.value)} inputMode="numeric" className="mt-1 w-32 rounded-xl border border-stone-300 bg-white px-3 py-2" />
            </label>
          ) : null}
          <label className="block text-sm">
            <span className="font-medium">What is it for?</span>
            <input value={purpose} onChange={(e) => setPurpose(e.target.value)} maxLength={200} className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2" />
          </label>
          <button type="submit" className="rounded-full bg-stone-900 px-6 py-3 text-white hover:bg-black">
            Apply
          </button>
        </form>
      ) : (
        <p className="text-stone-600">Credit is not yet available.</p>
      )}
    </section>
  );
}

function FacilityCard({ f, name, onAccept, onMove }: {
  f: Facility;
  name: string;
  onAccept: (id: string) => Promise<void>;
  onMove: (id: string, kind: "draw" | "repay", value: string) => Promise<void>;
}) {
  const [value, setValue] = useState("");
  const next = f.schedule.find((i) => i.status === "DUE" || i.status === "ARREARS");
  return (
    <li className="rounded-[2rem] bg-stone-100 p-8" aria-label={name}>
      <div className="flex items-baseline justify-between gap-2">
        <p className="text-sm text-stone-500">{name}</p>
        <p className="text-sm font-medium">{STATUS[f.status] ?? f.status}</p>
      </div>
      <p className="mt-1 text-3xl font-light tabular-nums">
        {formatMinor(f.status === "ACTIVE" || f.status === "SETTLED" ? f.outstanding_minor : f.approved_minor !== "0" ? f.approved_minor : f.requested_minor, "ZAR")}
      </p>
      <p className="text-sm text-stone-500">{f.status === "ACTIVE" ? "owed" : f.status === "APPROVED" ? "offered" : "asked for"}</p>
      {f.default_notice ? (
        <div className="mt-3 rounded-xl border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-900" role="alert">
          <p className="font-medium">
            Default notice: {formatMinor(f.default_notice.arrears_minor, "ZAR")} is {f.default_notice.days_past_due} days overdue. Please respond by{" "}
            {f.default_notice.respond_by}.
          </p>
          <p className="mt-1">{f.default_notice.text}</p>
        </div>
      ) : null}
      {f.days_past_due && !f.default_notice ? (
        <p className="mt-2 rounded-xl bg-red-50 px-3 py-2 text-sm text-red-900" role="alert">
          A payment is {f.days_past_due} days overdue. Add money to your current account and we will collect it.
        </p>
      ) : null}
      {f.status === "APPROVED" ? (
        <div className="mt-4 space-y-3 text-sm">
          <p>
            Total you will repay: <strong className="tabular-nums">{formatMinor(f.total_repayable_minor, "ZAR")}</strong>
          </p>
          <button type="button" onClick={() => onAccept(f.facility_id)} className="rounded-full bg-stone-900 px-6 py-3 text-white hover:bg-black">
            Accept the offer
          </button>
        </div>
      ) : null}
      {f.status === "ACTIVE" || f.status === "SETTLED" ? (
        <p className="mt-3 text-sm">
          <a className="underline" href={`/api/v1/facilities/${f.facility_id}/statement?period=${new Date().toISOString().slice(0, 7)}`}>
            This month&apos;s statement
          </a>
        </p>
      ) : null}
      {f.status === "ACTIVE" ? (
        <div className="mt-4 space-y-3 text-sm">
          {f.product_id === "overdraft" ? <p>Available to draw: {formatMinor(f.available_minor, "ZAR")}</p> : null}
          {next ? (
            <p>
              Next payment {formatMinor(String(BigInt(next.principal_minor) + BigInt(next.interest_minor)), "ZAR")} on {next.due_on}
            </p>
          ) : null}
          <div className="flex flex-wrap items-end gap-2">
            <label>
              <span className="block font-medium">Amount (R)</span>
              <input value={value} onChange={(e) => setValue(e.target.value)} inputMode="decimal" className="mt-1 w-36 rounded-xl border border-stone-300 bg-white px-3 py-2" />
            </label>
            {f.product_id === "overdraft" ? (
              <button type="button" onClick={() => onMove(f.facility_id, "draw", value)} className="rounded-full bg-stone-900 px-5 py-2 text-white">
                Draw
              </button>
            ) : null}
            <button type="button" onClick={() => onMove(f.facility_id, "repay", value)} className="rounded-full border border-stone-900 px-5 py-2">
              Repay
            </button>
          </div>
        </div>
      ) : null}
    </li>
  );
}
