"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor } from "@/lib/money";

type Entry = components["schemas"]["ActivityEntry"];

const PAGE = 50;

export default function ActivityPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const accountId = decodeURIComponent(params.id);
  const [entries, setEntries] = useState<Entry[]>([]);
  const [next, setNext] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Loads one page after a cursor. The first page replaces what is shown; later pages are appended.
  const load = useCallback(
    async (after: number, replace: boolean) => {
      setLoading(true);
      const { data, error, response } = await api.GET("/v1/accounts/{account}/activity", {
        params: { path: { account: accountId }, query: { after, limit: PAGE } },
      });
      setLoading(false);
      if (response.status === 401) return router.push("/sign-in");
      if (error || !data) return setError(errorText(error, "The activity could not be loaded. Try again."));
      setEntries((prev) => (replace ? data.entries : [...prev, ...data.entries]));
      setNext(data.next_after);
      setHasMore(data.entries.length === PAGE);
    },
    [accountId, router],
  );

  useEffect(() => {
    void load(0, true);
  }, [load]);

  return (
    <section aria-labelledby="activity-heading" className="space-y-4">
      <p>
        <Link href="/accounts" className="text-sm text-sky-700 underline">
          Back to accounts
        </Link>
      </p>
      <h1 id="activity-heading" className="text-2xl font-semibold">
        Activity
      </h1>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {!loading && !error && entries.length === 0 ? <p className="text-stone-600">No activity yet.</p> : null}
      {entries.length > 0 ? (
        <table className="w-full border-collapse text-left text-sm">
          <thead>
            <tr className="border-b border-stone-200 text-stone-500">
              <th className="py-2 font-medium">When</th>
              <th className="py-2 font-medium">Direction</th>
              <th className="py-2 text-right font-medium">Amount</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e) => (
              <tr key={e.seq} className="border-b border-stone-100">
                <td className="py-2">{new Date(e.at).toLocaleString("en-ZA")}</td>
                <td className="py-2">{e.direction === "IN" ? "In" : "Out"}</td>
                <td className="py-2 text-right tabular-nums">{formatMinor(e.amount_minor, e.asset)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
      {hasMore ? (
        <button
          type="button"
          onClick={() => void load(next, false)}
          disabled={loading}
          className="rounded-md border border-stone-300 bg-white px-4 py-2 text-sm hover:bg-stone-100 disabled:opacity-50"
        >
          {loading ? "Loading…" : "Load more"}
        </button>
      ) : null}
    </section>
  );
}
