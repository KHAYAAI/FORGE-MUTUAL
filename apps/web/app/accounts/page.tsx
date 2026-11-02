"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor } from "@/lib/money";

type Account = components["schemas"]["Account"];

export default function AccountsPage() {
  const router = useRouter();
  const [accounts, setAccounts] = useState<Account[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    api.GET("/v1/accounts").then(({ data, error, response }) => {
      if (!live) return;
      if (response.status === 401) return router.push("/sign-in");
      if (error) return setError(errorText(error, "Your accounts could not be loaded. Try again."));
      setAccounts(data?.accounts ?? []);
    });
    return () => {
      live = false;
    };
  }, [router]);

  if (error) return <Notice tone="error">{error}</Notice>;
  if (accounts === null) return <p className="text-stone-500">Loading your accounts…</p>;
  if (accounts.length === 0) {
    return <p className="text-stone-600">No accounts yet. Your accounts will appear here once they are opened.</p>;
  }

  return (
    <section aria-labelledby="accounts-heading" className="space-y-6">
      <h1 id="accounts-heading" className="text-2xl font-semibold">
        Your accounts
      </h1>
      <ul className="grid gap-4 sm:grid-cols-2">
        {accounts.map((a) => (
          <li key={a.account_id} className="rounded-lg border border-stone-200 bg-white p-5 shadow-sm">
            <div className="flex items-start justify-between gap-2">
              <p className="text-sm font-medium text-stone-500">{a.asset} account</p>
              <Link href={`/accounts/${encodeURIComponent(a.account_id)}`} className="text-sm text-sky-700 underline">
                Activity
              </Link>
            </div>
            <p className="mt-2 text-3xl font-semibold tabular-nums" data-testid="balance">
              {formatMinor(a.available_minor, a.asset)}
            </p>
            {/* Every balance names the entity that holds it and whether it is insured. */}
            <dl className="mt-4 space-y-1 text-sm text-stone-700">
              <div className="flex gap-2">
                <dt className="text-stone-500">Held by</dt>
                <dd data-testid="holding-entity">{a.holding_entity}</dd>
              </div>
              <div className="flex gap-2">
                <dt className="text-stone-500">Insurance</dt>
                <dd>
                  <span
                    data-testid="insured"
                    className={
                      a.insured
                        ? "rounded bg-emerald-100 px-2 py-0.5 text-emerald-900"
                        : "rounded bg-amber-100 px-2 py-0.5 text-amber-900"
                    }
                  >
                    {a.insured ? "Insured" : "Not insured"}
                  </span>
                </dd>
              </div>
              {a.as_of ? (
                <div className="flex gap-2">
                  <dt className="text-stone-500">As of</dt>
                  <dd>{new Date(a.as_of).toLocaleString("en-ZA")}</dd>
                </div>
              ) : null}
            </dl>
          </li>
        ))}
      </ul>
    </section>
  );
}
