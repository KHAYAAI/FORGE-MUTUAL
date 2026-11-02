"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor } from "@/lib/money";

type Batch = components["schemas"]["Batch"];

const STATE_LABEL: Record<Batch["state"], string> = {
  BATCH_STATE_DRAFT: "Waiting for approval",
  BATCH_STATE_RUNNING: "Sending",
  BATCH_STATE_COMPLETED: "Completed",
  BATCH_STATE_COMPLETED_WITH_EXCEPTIONS: "Completed with exceptions",
};

const ITEM_LABEL: Record<string, string> = {
  ITEM_STATE_PENDING: "Not yet sent",
  ITEM_STATE_SENT: "Sent, awaiting payment",
  ITEM_STATE_SETTLED: "Paid",
  ITEM_STATE_FAILED: "Failed",
  ITEM_STATE_UNCERTAIN: "Outcome unknown",
};

export default function PayoutBatchPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const batchId = decodeURIComponent(params.id);
  const [batch, setBatch] = useState<Batch | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    const { data, error, response } = await api.GET("/v1/payout-batches/{batch}", {
      params: { path: { batch: batchId } },
    });
    if (response.status === 401) return router.push("/sign-in");
    if (error || !data) return setError(errorText(error, "The batch could not be loaded."));
    setBatch(data);
  }, [batchId, router]);

  useEffect(() => {
    void load();
  }, [load]);

  async function approve() {
    setError(null);
    setBusy(true);
    const { data, error, response } = await api.POST("/v1/payout-batches/{batch}/approve", {
      params: { path: { batch: batchId } },
    });
    setBusy(false);
    if (response.status === 401) return router.push("/sign-in");
    if (error || !data) return setError(errorText(error, "The batch was not approved. Try again."));
    setBatch(data);
  }

  if (!batch) {
    return error ? <Notice tone="error">{error}</Notice> : <p className="text-stone-500">Loading the batch…</p>;
  }

  return (
    <section aria-labelledby="batch-heading" className="space-y-6">
      <p>
        <Link href="/accounts" className="text-sm text-sky-700 underline">
          Back to accounts
        </Link>
      </p>
      <h1 id="batch-heading" className="text-2xl font-semibold">
        Bulk payout
      </h1>
      {error ? <Notice tone="error">{error}</Notice> : null}

      <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm sm:max-w-md">
        <dt className="text-stone-500">Status</dt>
        <dd data-testid="batch-state">{STATE_LABEL[batch.state]}</dd>
        <dt className="text-stone-500">Total</dt>
        <dd className="tabular-nums">{formatMinor(batch.total_minor, batch.asset)}</dd>
        <dt className="text-stone-500">Created by</dt>
        <dd>{batch.created_by}</dd>
        <dt className="text-stone-500">Approved by</dt>
        <dd>{batch.approved_by || "Not yet approved"}</dd>
      </dl>

      {batch.state === "BATCH_STATE_DRAFT" ? (
        <div className="space-y-2">
          <p className="text-sm text-stone-700">
            Only a different person can approve this batch. Approving it starts the payments.
          </p>
          <button
            type="button"
            onClick={approve}
            disabled={busy}
            className="rounded-md bg-emerald-700 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-600 disabled:opacity-50"
          >
            {busy ? "Approving…" : "Approve and send"}
          </button>
        </div>
      ) : null}

      <table className="w-full border-collapse text-left text-sm">
        <thead>
          <tr className="border-b border-stone-200 text-stone-500">
            <th className="py-2 font-medium">Item</th>
            <th className="py-2 text-right font-medium">Amount</th>
            <th className="py-2 font-medium">Status</th>
          </tr>
        </thead>
        <tbody>
          {batch.items.map((it) => (
            <tr key={it.item_key} className="border-b border-stone-100 align-top">
              <td className="py-2 font-mono">{it.item_key}</td>
              <td className="py-2 text-right tabular-nums">{formatMinor(it.amount_minor, batch.asset)}</td>
              <td className="py-2">
                {ITEM_LABEL[it.state] ?? it.state}
                {it.reason ? <p className="text-xs text-stone-500">{it.reason}</p> : null}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
