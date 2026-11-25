"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useCallback, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { CardArt } from "@/components/Visuals";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor, parseToMinor } from "@/lib/money";

type Card = components["schemas"]["Card"];
type Account = components["schemas"]["Account"];
type Authorisation = components["schemas"]["Authorisation"];

const key = () => crypto.randomUUID();
const STATUS: Record<string, string> = { ACTIVE: "Active", FROZEN: "Frozen", CLOSED: "Closed" };
const OUTCOME: Record<string, string> = {
  HELD: "Pending", CAPTURED: "Paid", REVERSED: "Cancelled", REFUNDED: "Refunded", DECLINED: "Declined", EXPIRED: "Expired", EXCEPTION: "Being checked",
};
const REASON: Record<string, string> = {
  CARD_NOT_ACTIVE: "The card was frozen or closed.", CURRENCY_NOT_SUPPORTED: "Only rand purchases are supported.",
  MERCHANT_CATEGORY_BLOCKED: "This type of merchant is not allowed.", LIMIT_PER_TRANSACTION: "Over the limit for one purchase.",
  LIMIT_DAILY: "Over the daily limit.", INSUFFICIENT_FUNDS: "Not enough money in the account.",
  TEMPORARILY_UNAVAILABLE: "We could not check just now. Please try again.", ACCOUNT_RESTRICTED: "This action is not available on your account.",
};

// Virtual debit cards on your own rand account. The card number is shown on the card processor's page, never here.
export default function CardsPage() {
  const router = useRouter();
  const [cards, setCards] = useState<Card[] | null>(null);
  const [offered, setOffered] = useState(true);
  const [account, setAccount] = useState<Account | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);
  const [label, setLabel] = useState("");

  const load = useCallback(async () => {
    const [c, a] = await Promise.all([api.GET("/v1/cards"), api.GET("/v1/accounts")]);
    if (a.response.status === 401 || c.response.status === 401) return router.push("/sign-in");
    if (c.response.status === 404) return setOffered(false);
    setCards(c.data?.cards ?? []);
    setAccount((a.data?.accounts ?? []).find((x) => x.asset === "ZAR") ?? null);
  }, [router]);

  useEffect(() => {
    void load();
  }, [load]);

  const fail = (e: unknown, status: number) => setError(errorText(e, `Request failed (${status}).`));

  async function issue(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setDone(null);
    if (!account) return setError("You need a rand account to get a card.");
    if (!label.trim()) return setError("Give the card a name, for example Subscriptions.");
    const res = await api.POST("/v1/cards", { params: { header: { "Idempotency-Key": key() } }, body: { account_id: account.account_id, label: label.trim() } });
    if (!res.response.ok) return fail(res.error, res.response.status);
    setDone("Your card is ready.");
    setLabel("");
    await load();
  }

  async function setStatus(id: string, status: "ACTIVE" | "FROZEN" | "CLOSED") {
    setError(null);
    setDone(null);
    const res = await api.POST("/v1/cards/{card}/status", { params: { path: { card: id } }, body: { status } });
    if (!res.response.ok) return fail(res.error, res.response.status);
    setDone(status === "FROZEN" ? "The card is frozen." : status === "ACTIVE" ? "The card is active again." : "The card is closed.");
    await load();
  }

  async function setLimits(id: string, perTxn: string, daily: string) {
    setError(null);
    setDone(null);
    const a = parseToMinor(perTxn);
    const b = parseToMinor(daily);
    if (!a || !b) return setError("Enter both limits as amounts greater than zero.");
    const res = await api.POST("/v1/cards/{card}/limits", { params: { path: { card: id } }, body: { per_transaction_limit_minor: a, daily_limit_minor: b } });
    if (!res.response.ok) return fail(res.error, res.response.status);
    setDone("Limits saved.");
    await load();
  }

  async function show(id: string) {
    setError(null);
    const res = await api.POST("/v1/cards/{card}/display", { params: { path: { card: id } } });
    if (!res.response.ok || !res.data) return fail(res.error, res.response.status);
    window.open(res.data.url, "_blank", "noopener,noreferrer");
  }

  if (!offered)
    return (
      <section className="space-y-4" aria-labelledby="cards-heading">
        <h1 id="cards-heading" className="text-3xl font-semibold tracking-tight">
          Cards
        </h1>
        <p className="max-w-2xl text-stone-600">Cards are not yet available.</p>
      </section>
    );

  return (
    <section className="space-y-10" aria-labelledby="cards-heading">
      <div>
        <h1 id="cards-heading" className="text-3xl font-semibold tracking-tight">
          Cards
        </h1>
        <p className="mt-2 max-w-2xl text-stone-600">
          Virtual debit cards on your own rand account. Every purchase is checked against your limits and your balance when you make it, and
          the money is set aside straight away. We never see your card number: it is shown on the card provider&apos;s own page.
        </p>
      </div>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {done ? <Notice tone="success">{done}</Notice> : null}
      {cards === null ? <p className="text-stone-500">Loading…</p> : null}
      {cards?.length ? (
        <ul className="grid gap-4 md:grid-cols-2 2xl:grid-cols-3">
          {cards.map((c) => (
            <CardItem key={c.card_id} card={c} onStatus={setStatus} onLimits={setLimits} onShow={show} />
          ))}
        </ul>
      ) : null}
      {cards ? (
        <form onSubmit={issue} className="grid max-w-3xl gap-6 rounded-2xl border border-stone-200 bg-white p-5 shadow-sm sm:p-8 md:grid-cols-2 md:items-center" aria-label="Get a card">
          <div className="order-last md:order-first"><CardArt label={label} skin={(cards?.length ?? 0)} /></div>
          <div className="space-y-4">
          <h2 className="text-2xl font-semibold tracking-tight">Get a card</h2>
          <label className="block text-sm">
            <span className="font-medium">Name</span>
            <input value={label} onChange={(e) => setLabel(e.target.value)} maxLength={60} className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2" />
          </label>
          <button type="submit" className="rounded-full bg-stone-900 px-6 py-3 text-white hover:bg-black">
            Get card
          </button>
          </div>
        </form>
      ) : null}
    </section>
  );
}

function CardItem({ card, onStatus, onLimits, onShow }: {
  card: Card;
  onStatus: (id: string, s: "ACTIVE" | "FROZEN" | "CLOSED") => Promise<void>;
  onLimits: (id: string, perTxn: string, daily: string) => Promise<void>;
  onShow: (id: string) => Promise<void>;
}) {
  const [perTxn, setPerTxn] = useState((Number(card.per_transaction_limit_minor ?? "0") / 100).toString());
  const [daily, setDaily] = useState((Number(card.daily_limit_minor ?? "0") / 100).toString());
  const [auths, setAuths] = useState<Authorisation[] | null>(null);
  const closed = card.status === "CLOSED";
  async function toggleAuths() {
    if (auths) return setAuths(null);
    const res = await api.GET("/v1/cards/{card}/authorisations", { params: { path: { card: card.card_id } } });
    setAuths(res.data?.authorisations ?? []);
  }
  return (
    <li className="rounded-2xl border border-stone-200 bg-white p-5 shadow-sm sm:p-6" aria-label={card.label}>
      <CardArt
        label={card.label ?? ""}
        last4={card.last4}
        expiry={`${String(card.expiry_month ?? "").padStart(2, "0")}/${String(card.expiry_year ?? "").slice(-2)}`}
        frozen={card.status !== "ACTIVE"}
        skin={[...card.card_id].reduce((n, ch) => n + ch.charCodeAt(0), 0)}
      />
      <div className="mt-5 flex items-baseline justify-between gap-2">
        <p className="font-medium">{card.label}</p>
        <p className="text-sm font-medium text-stone-600">{STATUS[card.status] ?? card.status}</p>
      </div>
      <p className="text-sm text-stone-500">
        Expires {String(card.expiry_month ?? "").padStart(2, "0")}/{card.expiry_year}
        {card.network === "SIMULATED" ? " · test card" : ""}
      </p>
      <p className="mt-2 text-sm">
        Limits: {formatMinor(card.per_transaction_limit_minor ?? "0", "ZAR")} a purchase, {formatMinor(card.daily_limit_minor ?? "0", "ZAR")} a day
      </p>
      {!closed ? (
        <div className="mt-4 flex flex-wrap gap-2 text-sm">
          {card.status === "ACTIVE" ? (
            <>
              <button type="button" onClick={() => onShow(card.card_id)} className="rounded-full border border-stone-900 px-4 py-2">
                Show card details
              </button>
              <button type="button" onClick={() => onStatus(card.card_id, "FROZEN")} className="rounded-full border border-stone-900 px-4 py-2">
                Freeze
              </button>
            </>
          ) : (
            <button type="button" onClick={() => onStatus(card.card_id, "ACTIVE")} className="rounded-full bg-stone-900 px-4 py-2 text-white">
              Unfreeze
            </button>
          )}
          <button type="button" onClick={() => onStatus(card.card_id, "CLOSED")} className="rounded-full border border-red-700 px-4 py-2 text-red-800">
            Close card
          </button>
        </div>
      ) : null}
      {!closed ? (
        <div className="mt-4 flex flex-wrap items-end gap-2 text-sm">
          <label>
            <span className="block font-medium">Per purchase (R)</span>
            <input value={perTxn} onChange={(e) => setPerTxn(e.target.value)} inputMode="decimal" className="mt-1 w-32 rounded-xl border border-stone-300 bg-white px-3 py-2" />
          </label>
          <label>
            <span className="block font-medium">Per day (R)</span>
            <input value={daily} onChange={(e) => setDaily(e.target.value)} inputMode="decimal" className="mt-1 w-32 rounded-xl border border-stone-300 bg-white px-3 py-2" />
          </label>
          <button type="button" onClick={() => onLimits(card.card_id, perTxn, daily)} className="rounded-full border border-stone-900 px-4 py-2">
            Save limits
          </button>
        </div>
      ) : null}
      <p className="mt-4 text-sm">
        <button type="button" className="underline" onClick={toggleAuths} aria-expanded={auths !== null}>
          {auths ? "Hide purchases" : "Show purchases"}
        </button>
      </p>
      {auths ? (
        auths.length ? (
          <ul className="mt-2 space-y-1 text-sm">
            {auths.map((a) => (
              <li key={a.auth_id} className="flex justify-between gap-2">
                <span>
                  {a.merchant_name || "Purchase"} · {OUTCOME[a.status] ?? a.status}
                  {a.decline_reason ? ` (${REASON[a.decline_reason] ?? a.decline_reason})` : ""}
                </span>
                <span className="tabular-nums">{formatMinor(a.amount_minor, "ZAR")}</span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="mt-2 text-sm text-stone-500">No purchases yet.</p>
        )
      ) : null}
    </li>
  );
}
