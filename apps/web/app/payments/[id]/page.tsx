"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor } from "@/lib/money";

type Payment = components["schemas"]["Payment"];

const STATE_LABEL: Record<Payment["state"], string> = {
  PAYMENT_STATE_RUNNING: "In progress",
  PAYMENT_STATE_AWAITING_APPROVAL: "Waiting for approval",
  PAYMENT_STATE_RESERVED: "Reserved, awaiting the bank",
  PAYMENT_STATE_COMPLETED: "Paid",
  PAYMENT_STATE_DECLINED: "Declined",
  PAYMENT_STATE_FAILED: "Failed",
  PAYMENT_STATE_CANCELLED: "Cancelled",
  PAYMENT_STATE_REPAIR_REQUIRED: "Being checked. We will contact you if we need anything.",
};

export default function PaymentStatusPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const paymentId = decodeURIComponent(params.id);
  const [payment, setPayment] = useState<Payment | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const { data, error, response } = await api.GET("/v1/payments/{payment}", {
      params: { path: { payment: paymentId } },
    });
    if (response.status === 401) return router.push("/sign-in");
    if (error || !data) return setError(errorText(error, "The payment could not be loaded."));
    setPayment(data);
  }, [paymentId, router]);

  useEffect(() => {
    void load();
  }, [load]);

  if (error) return <Notice tone="error">{error}</Notice>;
  if (!payment) return <p className="text-stone-500">Loading the payment…</p>;

  return (
    <section className="max-w-xl space-y-4">
      <p>
        <Link href="/accounts" className="text-sm text-sky-700 underline">
          Back to accounts
        </Link>
      </p>
      <h1 className="text-2xl font-semibold">Payment</h1>
      <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
        <dt className="text-stone-500">Status</dt>
        <dd data-testid="payment-state">{STATE_LABEL[payment.state]}</dd>
        {payment.amount_minor ? (
          <>
            <dt className="text-stone-500">Amount</dt>
            <dd className="tabular-nums">{formatMinor(payment.amount_minor, payment.asset ?? "ZAR")}</dd>
          </>
        ) : null}
        {payment.reason ? (
          <>
            <dt className="text-stone-500">Reason</dt>
            <dd>{payment.reason}</dd>
          </>
        ) : null}
      </dl>
      <button type="button" onClick={() => void load()} className="text-sm text-sky-700 underline">
        Refresh
      </button>
    </section>
  );
}
