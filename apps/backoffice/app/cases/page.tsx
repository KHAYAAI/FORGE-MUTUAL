"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

type Case = components["schemas"]["Case"];

export default function CasesPage() {
  const router = useRouter();
  const [cases, setCases] = useState<Case[] | null>(null);
  const [all, setAll] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    setCases(null);
    api.GET("/v1/cases", { params: { query: { status: all ? "all" : "open" } } }).then(({ data, error, response }) => {
      if (!live) return;
      if (response.status === 401) return router.push("/sign-in");
      if (error || !data) return setError(errorText(error, "The cases could not be loaded. Try again."));
      setError(null);
      setCases(data.cases);
    });
    return () => {
      live = false;
    };
  }, [all, router]);

  return (
    <section aria-labelledby="cases-heading" className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 id="cases-heading" className="text-2xl font-semibold">
          Compliance cases
        </h1>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={all} onChange={(e) => setAll(e.target.checked)} />
          Include closed cases
        </label>
      </div>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {cases === null && !error ? <p className="text-stone-500">Loading cases…</p> : null}
      {cases?.length === 0 ? <p className="text-stone-600">No cases to show.</p> : null}
      {cases && cases.length > 0 ? (
        <table className="w-full border-collapse overflow-hidden rounded-lg bg-white text-left text-sm shadow-sm">
          <thead className="bg-stone-50 text-stone-500">
            <tr>
              <th className="px-4 py-2 font-medium">Case</th>
              <th className="px-4 py-2 font-medium">Rule</th>
              <th className="px-4 py-2 font-medium">Severity</th>
              <th className="px-4 py-2 font-medium">Due</th>
              <th className="px-4 py-2 font-medium">Status</th>
            </tr>
          </thead>
          <tbody>
            {cases.map((c) => (
              <tr key={c.case_id} className="border-t border-stone-200">
                <td className="px-4 py-2">
                  <Link href={`/cases/${encodeURIComponent(c.case_id)}`} className="font-mono text-sky-700 underline">
                    {c.case_id.slice(0, 8)}
                  </Link>
                </td>
                <td className="px-4 py-2">{c.rule}</td>
                <td className="px-4 py-2">{c.severity}</td>
                <td className="px-4 py-2">
                  {new Date(c.sla_due_at ?? "").toLocaleString("en-ZA")}
                  {c.overdue ? (
                    <span data-testid="overdue" className="ml-2 rounded bg-red-100 px-2 py-0.5 text-xs text-red-900">
                      Overdue
                    </span>
                  ) : null}
                </td>
                <td className="px-4 py-2">{c.status === "OPEN" ? "Open" : "Closed"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
    </section>
  );
}
