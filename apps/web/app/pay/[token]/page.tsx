"use client";

import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import type { components } from "@/lib/api/schema";
import { formatMinor } from "@/lib/money";

type Checkout = components["schemas"]["Checkout"];

// The public pay page. The payer sees what they are paying and to whom, then pays on the acquirer's own page: card
// details are never entered here.
export default function PayPage() {
  const token = useParams<{ token: string }>().token;
  const [co, setCo] = useState<Checkout | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    fetch(`/api/checkout/${encodeURIComponent(token)}`, { cache: "no-store" })
      .then(async (r) => (r.ok ? setCo((await r.json()) as Checkout) : setError("This payment link is not valid.")))
      .catch(() => setError("Payments are not reachable. Try again shortly."));
  }, [token]);

  async function pay() {
    setBusy(true);
    const r = await fetch(`/api/checkout/${encodeURIComponent(token)}/start`, { method: "POST" });
    const body = (await r.json().catch(() => null)) as { checkout_url?: string; error?: { message?: string } } | null;
    setBusy(false);
    if (!r.ok || !body?.checkout_url) return setError(body?.error?.message ?? "The payment could not be started.");
    window.location.assign(body.checkout_url);
  }

  return (
    <section className="mx-auto max-w-md space-y-6 py-10" aria-labelledby="pay-heading">
      {error ? <p role="alert" className="rounded-xl bg-red-50 px-4 py-3 text-red-900">{error}</p> : null}
      {co ? (
        <div className="rounded-2xl border border-stone-200 bg-white p-5 shadow-sm sm:p-8">
          <p className="text-sm text-stone-500">Pay {co.merchant_name}</p>
          <h1 id="pay-heading" className="mt-2 text-5xl font-light tabular-nums tracking-tight">
            {formatMinor(co.amount_minor, "ZAR")}
          </h1>
          <p className="mt-3 text-stone-700">{co.description}</p>
          {co.reference ? <p className="text-sm text-stone-500">Reference {co.reference}</p> : null}
          {co.status === "OPEN" ? (
            <button type="button" onClick={pay} disabled={busy} className="mt-8 w-full rounded-full bg-stone-900 px-6 py-3 text-white hover:bg-black disabled:opacity-50">
              {busy ? "Opening the secure checkout…" : "Pay by card"}
            </button>
          ) : (
            <p className="mt-8 rounded-xl bg-white px-4 py-3 text-center" role="status">
              {co.status === "PAID" ? "Paid. Thank you." : co.status === "EXPIRED" ? "This payment link has expired." : "This payment link was cancelled."}
            </p>
          )}
          <p className="mt-6 text-xs text-stone-500">You pay on our card partner&apos;s secure page. Forge never sees your card details.</p>
        </div>
      ) : null}
    </section>
  );
}
