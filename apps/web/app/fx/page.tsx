"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor, parseToMinor } from "@/lib/money";

type Account = components["schemas"]["Account"];
type Quote = components["schemas"]["Quote"];
type Asset = "ZAR" | "USD" | "EUR" | "GBP";

// Every conversion is the rand against one foreign currency, either way.
const PAIRS: { sell: Asset; buy: Asset }[] = [
  { sell: "ZAR", buy: "USD" },
  { sell: "ZAR", buy: "EUR" },
  { sell: "ZAR", buy: "GBP" },
  { sell: "USD", buy: "ZAR" },
  { sell: "EUR", buy: "ZAR" },
  { sell: "GBP", buy: "ZAR" },
];

export default function FxPage() {
  const router = useRouter();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [pair, setPair] = useState("ZAR-USD");
  const [amount, setAmount] = useState("");
  const [quote, setQuote] = useState<Quote | null>(null);
  const [acknowledged, setAcknowledged] = useState(false);
  const [sellAccount, setSellAccount] = useState("");
  const [buyAccount, setBuyAccount] = useState("");
  const [deal, setDeal] = useState<{ id: string; state: string } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [sellAsset, buyAsset] = pair.split("-") as [Asset, Asset];
  const sellOptions = accounts.filter((a) => a.asset === sellAsset);
  const buyOptions = accounts.filter((a) => a.asset === buyAsset);

  useEffect(() => {
    api.GET("/v1/accounts").then(({ data, response }) => {
      if (response.status === 401) return router.push("/sign-in");
      setAccounts(data?.accounts ?? []);
    });
  }, [router]);

  // Keep the selected accounts in step with the currencies chosen.
  useEffect(() => {
    setSellAccount(sellOptions[0]?.account_id ?? "");
    setBuyAccount(buyOptions[0]?.account_id ?? "");
    setQuote(null);
    setAcknowledged(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pair, accounts]);

  const expired = quote !== null && Date.parse(quote.expires_at) <= Date.now();

  async function getQuote(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setDeal(null);
    setAcknowledged(false);
    const minor = parseToMinor(amount);
    if (!minor) return setError("Enter an amount greater than zero, with at most two decimal places.");
    setBusy(true);
    const { data, error, response } = await api.POST("/v1/fx/quotes", {
      body: { sell_asset: sellAsset, buy_asset: buyAsset, sell_amount_minor: minor },
    });
    setBusy(false);
    if (response.status === 401) return router.push("/sign-in");
    if (error || !data) return setError(errorText(error, "No quote is available right now. Try again."));
    setQuote(data);
  }

  async function book() {
    if (!quote) return;
    setError(null);
    setBusy(true);
    const { data, error, response } = await api.POST("/v1/fx/deals", {
      body: { quote_id: quote.quote_id, sell_account_id: sellAccount, buy_account_id: buyAccount },
    });
    setBusy(false);
    if (response.status === 401) return router.push("/sign-in");
    if (error || !data) return setError(errorText(error, "The conversion was not booked. Get a new quote and try again."));
    setDeal({ id: data.deal_id, state: data.state });
    setQuote(null);
  }

  return (
    <section className="max-w-xl space-y-6">
      <h1 className="text-4xl font-light tracking-tight">Convert currency</h1>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {deal ? (
        <Notice tone="success">
          Conversion booked. Reference <span className="font-mono">{deal.id}</span>, status {deal.state}.
        </Notice>
      ) : null}

      <form onSubmit={getQuote} className="space-y-4" aria-label="Get a quote">
        <label className="block text-sm">
          <span className="font-medium">I am converting</span>
          <select
            name="pair"
            value={pair}
            onChange={(e) => setPair(e.target.value)}
            className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2 text-sm"
          >
            {PAIRS.map((p) => (
              <option key={`${p.sell}-${p.buy}`} value={`${p.sell}-${p.buy}`}>
                {p.sell} to {p.buy}
              </option>
            ))}
          </select>
        </label>
        <label className="block text-sm">
          <span className="font-medium">Amount ({sellAsset})</span>
          <input
            name="amount"
            inputMode="decimal"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            required
            className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2 text-sm tabular-nums"
          />
        </label>
        <button
          type="submit"
          disabled={busy}
          className="rounded-full bg-stone-900 px-6 py-3 text-base text-white hover:bg-black disabled:opacity-50"
        >
          Get a quote
        </button>
      </form>

      {quote ? (
        <div className="space-y-4 rounded-[2rem] bg-stone-100 p-8" aria-label="Your quote">
          <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
            <dt className="text-stone-500">You pay</dt>
            <dd className="tabular-nums">{formatMinor(quote.sell_amount_minor, quote.sell_asset)}</dd>
            <dt className="text-stone-500">You receive</dt>
            <dd className="tabular-nums">{formatMinor(quote.buy_amount_minor, quote.buy_asset)}</dd>
            <dt className="text-stone-500">Our spread</dt>
            <dd className="tabular-nums">
              {formatMinor(quote.spread_minor, quote.sell_asset)} ({quote.spread_bps / 100}%)
            </dd>
            <dt className="text-stone-500">Valid until</dt>
            <dd>{new Date(quote.expires_at).toLocaleTimeString("en-ZA")}</dd>
          </dl>
          <p className="rounded bg-stone-100 p-3 text-sm text-stone-800" data-testid="disclosure">
            {quote.disclosure}
          </p>
          {expired ? (
            <Notice tone="error">This quote has expired. Get a new quote to continue.</Notice>
          ) : (
            <>
              <label className="flex items-start gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={acknowledged}
                  onChange={(e) => setAcknowledged(e.target.checked)}
                  className="mt-1"
                />
                <span>I have read the disclosure above and accept this rate.</span>
              </label>
              <label className="block text-sm">
                <span className="font-medium">Pay from</span>
                <select
                  value={sellAccount}
                  onChange={(e) => setSellAccount(e.target.value)}
                  className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2 text-sm"
                >
                  {sellOptions.map((a) => (
                    <option key={a.account_id} value={a.account_id}>
                      {a.asset} account
                    </option>
                  ))}
                </select>
              </label>
              <label className="block text-sm">
                <span className="font-medium">Receive into</span>
                <select
                  value={buyAccount}
                  onChange={(e) => setBuyAccount(e.target.value)}
                  className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2 text-sm"
                >
                  {buyOptions.map((a) => (
                    <option key={a.account_id} value={a.account_id}>
                      {a.asset} account
                    </option>
                  ))}
                </select>
              </label>
              <button
                type="button"
                onClick={book}
                disabled={!acknowledged || busy || !sellAccount || !buyAccount}
                className="rounded-md bg-emerald-700 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-600 disabled:opacity-50"
              >
                Book this conversion
              </button>
            </>
          )}
        </div>
      ) : null}
    </section>
  );
}
