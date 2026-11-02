"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useState } from "react";
import Notice from "@/components/Notice";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export default function PaymentLookupPage() {
  const router = useRouter();
  const [id, setId] = useState("");
  const [error, setError] = useState<string | null>(null);

  function open(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const trimmed = id.trim();
    if (!UUID.test(trimmed)) return setError("Enter the payment's ID, a UUID.");
    router.push(`/payments/${trimmed}`);
  }

  return (
    <section className="max-w-xl space-y-6">
      <h1 className="text-2xl font-semibold">Find a payment</h1>
      {error ? <Notice tone="error">{error}</Notice> : null}
      <form onSubmit={open} className="flex gap-3" aria-label="Find a payment">
        <label className="flex-1 text-sm">
          <span className="sr-only">Payment ID</span>
          <input
            name="payment"
            value={id}
            onChange={(e) => setId(e.target.value)}
            placeholder="Payment ID"
            className="w-full rounded-md border border-stone-300 bg-white px-3 py-2 font-mono text-sm"
          />
        </label>
        <button type="submit" className="rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white hover:bg-stone-800">
          Open
        </button>
      </form>
    </section>
  );
}
