"use client";

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

type Offer = components["schemas"]["Offer"];
type Referral = components["schemas"]["Referral"];

const key = () => crypto.randomUUID();
const CATEGORY: Record<string, string> = { INVESTMENTS: "Investments", CRYPTO_ASSETS: "Crypto assets" };

// Products from licensed third parties. We introduce you; the provider provides the product, is licensed for it, and monitors it.
// The statement below comes from the server and is shown in full, unchanged. You must acknowledge it before we introduce you.
export default function InvestPage() {
  const router = useRouter();
  const [offers, setOffers] = useState<Offer[] | null>(null);
  const [offered, setOffered] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [referral, setReferral] = useState<Referral | null>(null);

  const load = useCallback(async () => {
    const res = await api.GET("/v1/offers");
    if (res.response.status === 401) return router.push("/sign-in");
    if (res.response.status === 404) return setOffered(false);
    setOffers(res.data?.offers ?? []);
  }, [router]);

  useEffect(() => {
    void load();
  }, [load]);

  async function continueTo(offer: Offer) {
    setError(null);
    const res = await api.POST("/v1/offers/{offer}/referrals", {
      params: { path: { offer: offer.offer_id }, header: { "Idempotency-Key": key() } },
      body: { acknowledged_third_party: true, statement_hash: offer.statement_hash },
    });
    if (!res.response.ok || !res.data) return setError(errorText(res.error, `Request failed (${res.response.status}).`));
    setReferral(res.data);
  }

  return (
    <section className="space-y-10" aria-labelledby="invest-heading">
      <div>
        <h1 id="invest-heading" className="text-4xl font-light tracking-tight">
          Invest
        </h1>
        <p className="mt-2 max-w-2xl text-stone-600">
          These products are provided, licensed and monitored by other companies, not by Forge Mutual. We can introduce you to them. We do not
          advise you, hold your money in them, or guarantee them.
        </p>
      </div>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {referral ? (
        <Notice tone="success">
          You are being introduced to {referral.provider_name}. Continue on their own site:{" "}
          <a className="underline" href={referral.handoff_url} rel="noopener noreferrer">
            go to {referral.provider_name}
          </a>
          .
        </Notice>
      ) : null}
      {!offered ? <p className="text-stone-600">Third-party products are not yet available.</p> : null}
      {offered && offers?.length === 0 ? <p className="text-stone-600">No third-party products are available at the moment.</p> : null}
      {offers?.length ? (
        <ul className="space-y-6">
          {offers.map((o) => (
            <OfferCard key={o.offer_id} offer={o} onContinue={continueTo} />
          ))}
        </ul>
      ) : null}
    </section>
  );
}

function OfferCard({ offer, onContinue }: { offer: Offer; onContinue: (o: Offer) => Promise<void> }) {
  const [ack, setAck] = useState(false);
  return (
    <li className="rounded-[2rem] bg-stone-100 p-8" aria-label={offer.title}>
      <p className="text-sm text-stone-500">
        {CATEGORY[offer.category ?? ""] ?? offer.category} · {offer.provider_name}
      </p>
      <h2 className="mt-1 text-2xl font-light tracking-tight">{offer.title}</h2>
      <p className="mt-2 text-stone-700">{offer.summary}</p>
      <div className="mt-4 rounded-xl border border-stone-400 bg-white p-4 text-sm" role="region" aria-label={`Important: about ${offer.title}`}>
        <p className="whitespace-pre-line">{offer.statement}</p>
      </div>
      <label className="mt-4 flex items-start gap-3 text-sm">
        <input type="checkbox" checked={ack} onChange={(e) => setAck(e.target.checked)} className="mt-1" />
        <span>
          I have read this. I understand that {offer.provider_name} provides, is licensed for and monitors this product, and that Forge Mutual
          does not.
        </span>
      </label>
      <button
        type="button"
        disabled={!ack}
        onClick={() => onContinue(offer)}
        className="mt-4 rounded-full bg-stone-900 px-6 py-3 text-white hover:bg-black disabled:opacity-40"
      >
        Continue to {offer.provider_name}
      </button>
    </li>
  );
}
