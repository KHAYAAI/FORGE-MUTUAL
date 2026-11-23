"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor } from "@/lib/money";
import { hasRole, useStaff } from "@/lib/useStaff";

type Payment = components["schemas"]["Payment"];

const STATE_LABEL: Record<Payment["state"], string> = {
  PAYMENT_STATE_RUNNING: "In progress",
  PAYMENT_STATE_AWAITING_APPROVAL: "Awaiting approval",
  PAYMENT_STATE_RESERVED: "Reserved, awaiting the rail",
  PAYMENT_STATE_COMPLETED: "Completed",
  PAYMENT_STATE_DECLINED: "Declined",
  PAYMENT_STATE_FAILED: "Failed",
  PAYMENT_STATE_CANCELLED: "Cancelled",
  PAYMENT_STATE_REPAIR_REQUIRED: "Repair required: the rail's outcome is unknown",
};

export default function StaffPaymentPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const paymentId = decodeURIComponent(params.id);
  const staff = useStaff(`/payments/${paymentId}`);
  const [payment, setPayment] = useState<Payment | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    const { data, error, response } = await api.GET("/v1/payments/{payment}", { params: { path: { payment: paymentId } } });
    if (response.status === 401) return router.push("/sign-in");
    if (error || !data) return setError(errorText(error, "The payment could not be loaded."));
    setPayment(data);
  }, [paymentId, router]);

  useEffect(() => {
    void load();
  }, [load]);

  async function decide(approve: boolean) {
    setError(null);
    setNotice(null);
    setBusy(true);
    const { error, response } = await api.POST("/v1/payments/{payment}/approval", {
      params: { path: { payment: paymentId } },
      body: { approve },
    });
    setBusy(false);
    if (response.status === 401) return router.push("/sign-in");
    if (error) return setError(errorText(error, "The decision was not recorded. Try again."));
    setNotice(approve ? "Payment approved." : "Payment refused.");
    await load();
  }

  async function repair(settled: boolean) {
    setError(null);
    setNotice(null);
    setBusy(true);
    const { error, response } = await api.POST("/v1/payments/{payment}/repair", {
      params: { path: { payment: paymentId } },
      body: { settled },
    });
    setBusy(false);
    if (response.status === 401) return router.push("/sign-in");
    if (error) return setError(errorText(error, "The outcome was not recorded. Try again."));
    setNotice(settled ? "Recorded: the rail confirmed the money moved." : "Recorded: the rail confirmed it did not move.");
    await load();
  }

  if (!payment) {
    return error ? <Notice tone="error">{error}</Notice> : <p className="text-stone-500">Loading the payment…</p>;
  }

  const canOperate = hasRole(staff ?? null, "operations");

  return (
    <section className="max-w-2xl space-y-6">
      <p>
        <Link href="/payments/lookup" className="text-sm text-sky-700 underline">
          Find another payment
        </Link>
      </p>
      <h1 className="text-2xl font-semibold">Payment</h1>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {notice ? <Notice tone="success">{notice}</Notice> : null}

      <dl className="grid grid-cols-2 gap-x-6 gap-y-2 rounded-lg bg-white p-5 text-sm shadow-sm">
        <dt className="text-stone-500">Status</dt>
        <dd data-testid="payment-state">{STATE_LABEL[payment.state]}</dd>
        {payment.amount_minor ? (
          <>
            <dt className="text-stone-500">Amount</dt>
            <dd className="tabular-nums">{formatMinor(payment.amount_minor, payment.asset ?? "ZAR")}</dd>
          </>
        ) : null}
        {payment.initiated_by ? (
          <>
            <dt className="text-stone-500">Initiated by</dt>
            <dd>{payment.initiated_by}</dd>
          </>
        ) : null}
        {payment.reason ? (
          <>
            <dt className="text-stone-500">Reason</dt>
            <dd>{payment.reason}</dd>
          </>
        ) : null}
      </dl>

      {payment.state === "PAYMENT_STATE_AWAITING_APPROVAL" ? (
        canOperate ? (
          <div className="space-y-2">
            <p className="text-sm text-stone-700">Approving sends the payment. Refusing cancels it before any money moves.</p>
            <div className="flex gap-3">
              <button
                type="button"
                disabled={busy}
                onClick={() => decide(true)}
                className="rounded-md bg-emerald-700 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-600 disabled:opacity-50"
              >
                Approve
              </button>
              <button
                type="button"
                disabled={busy}
                onClick={() => decide(false)}
                className="rounded-md border border-red-300 bg-white px-4 py-2 text-sm font-medium text-red-800 hover:bg-red-50 disabled:opacity-50"
              >
                Refuse
              </button>
            </div>
          </div>
        ) : (
          <Notice tone="info">An operations role is needed to decide this payment.</Notice>
        )
      ) : null}

      {payment.state === "PAYMENT_STATE_REPAIR_REQUIRED" ? (
        canOperate ? (
          <div className="space-y-2">
            <p className="text-sm text-stone-700">
              Check with the rail before recording an outcome. Record what the rail confirms, not what you expect.
            </p>
            <div className="flex flex-wrap gap-3">
              <button
                type="button"
                disabled={busy}
                onClick={() => repair(true)}
                className="rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white hover:bg-stone-800 disabled:opacity-50"
              >
                The rail confirms the money moved
              </button>
              <button
                type="button"
                disabled={busy}
                onClick={() => repair(false)}
                className="rounded-md border border-stone-400 bg-white px-4 py-2 text-sm font-medium text-stone-900 hover:bg-stone-100 disabled:opacity-50"
              >
                The rail confirms it did not move
              </button>
            </div>
          </div>
        ) : (
          <Notice tone="info">An operations role is needed to resolve this repair.</Notice>
        )
      ) : null}
    </section>
  );
}
