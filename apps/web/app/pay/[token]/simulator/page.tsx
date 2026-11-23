"use client";

import { useParams, useRouter } from "next/navigation";
import { useState } from "react";

// The acquirer simulator's checkout, for development. No card is asked for or charged: it only reports a result, the
// way a real acquirer's webhook would. Production refuses the simulator, so this page cannot complete a payment there.
export default function SimulatorPage() {
  const token = useParams<{ token: string }>().token;
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);

  async function report(approve: boolean) {
    const r = await fetch(`/api/checkout/${encodeURIComponent(token)}/simulate`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ approve }),
    });
    if (!r.ok) return setError("The simulator is not available.");
    router.push(`/pay/${token}`);
  }

  return (
    <section className="mx-auto max-w-md space-y-6 py-10" aria-labelledby="sim-heading">
      <p className="rounded-xl bg-amber-100 px-4 py-3 text-sm text-amber-900">
        Test checkout. This is the acquirer simulator: no card is asked for and nothing is charged.
      </p>
      <h1 id="sim-heading" className="text-3xl font-light tracking-tight">
        Simulated card payment
      </h1>
      {error ? <p role="alert" className="text-red-800">{error}</p> : null}
      <div className="flex gap-3">
        <button type="button" onClick={() => report(true)} className="rounded-full bg-stone-900 px-6 py-3 text-white">
          Approve test payment
        </button>
        <button type="button" onClick={() => report(false)} className="rounded-full border border-stone-900 px-6 py-3">
          Decline
        </button>
      </div>
    </section>
  );
}
