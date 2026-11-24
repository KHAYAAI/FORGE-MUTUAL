"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { hasRole, useStaff } from "@/lib/useStaff";

type Provider = {
  provider_id: string; name: string; licence_type: string; regulator: string; licence_number: string; status: string;
  licence_verified_at: string; licence_verified_by: string; licence_expires_at: string;
};
type AdminOffer = {
  offer: { offer_id: string; provider_id: string; provider_name: string; category: string; title: string; summary: string; statement: string; fee_disclosure: string };
  status: string; handoff_url: string; updated_by: string; visible_now: boolean;
};

const post = (path: string, body: unknown) =>
  fetch(path, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });

// Third-party providers, compliance's side. Every product here is provided, licensed and monitored by the provider, never by Forge.
// Two people are always involved: one registers a provider and another verifies its licence on the regulator's own register; one
// writes an offer and another reads the statement customers will see and switches it on. The server enforces both rules.
export default function PartnersPage() {
  const router = useRouter();
  const staff = useStaff("/partners");
  const [providers, setProviders] = useState<Provider[] | null>(null);
  const [offers, setOffers] = useState<AdminOffer[] | null>(null);
  const [offered, setOffered] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  async function load() {
    const [p, o] = await Promise.all([fetch("/api/v1/partners/providers", { cache: "no-store" }), fetch("/api/v1/partners/offers", { cache: "no-store" })]);
    if (p.status === 401) return router.push("/sign-in");
    if (p.status === 404) return setOffered(false);
    if (!p.ok) return setError(p.status === 403 ? "You need the compliance role to manage third-party providers." : "The providers could not be loaded.");
    setError(null);
    setProviders(((await p.json()) as { providers?: Provider[] }).providers ?? []);
    setOffers(o.ok ? (((await o.json()) as { offers?: AdminOffer[] }).offers ?? []) : []);
  }

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router]);

  async function act(path: string, body: unknown, ok: string) {
    setError(null);
    setNotice(null);
    const res = await post(path, body);
    const data = (await res.json().catch(() => null)) as { error?: { message?: string } } | null;
    if (!res.ok) return setError(data?.error?.message ?? "That was not recorded.");
    setNotice(ok);
    await load();
  }

  if (!offered)
    return (
      <section className="space-y-3" aria-labelledby="partners-heading">
        <h1 id="partners-heading" className="text-2xl font-semibold">
          Third-party providers
        </h1>
        <p className="text-stone-600">Third-party offers are not switched on.</p>
      </section>
    );

  const can = hasRole(staff ?? null, "compliance");
  return (
    <section aria-labelledby="partners-heading" className="space-y-8">
      <div>
        <h1 id="partners-heading" className="text-2xl font-semibold">
          Third-party providers
        </h1>
        <p className="mt-1 max-w-3xl text-sm text-stone-600">
          Every product is provided, licensed and monitored by the provider, never by Forge. A provider appears to customers only while its licence has been
          verified on the regulator&apos;s own register within the last twelve months. Registering and verifying are different people; writing an offer and
          switching it on are different people.
        </p>
      </div>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {notice ? <Notice tone="success">{notice}</Notice> : null}

      <div className="space-y-3">
        <h2 className="text-lg font-semibold">Providers</h2>
        {providers?.length === 0 ? <p className="text-sm text-stone-600">No providers are registered.</p> : null}
        <ul className="space-y-3">
          {(providers ?? []).map((p) => (
            <ProviderRow key={p.provider_id} p={p} can={can} onAct={act} />
          ))}
        </ul>
        {can ? <RegisterForm onAct={act} /> : null}
      </div>

      <div className="space-y-3">
        <h2 className="text-lg font-semibold">Offers</h2>
        {offers?.length === 0 ? <p className="text-sm text-stone-600">No offers are written.</p> : null}
        <ul className="space-y-3">
          {(offers ?? []).map((o) => (
            <OfferRow key={o.offer.offer_id} o={o} can={can} onAct={act} />
          ))}
        </ul>
        {can ? <OfferForm providers={providers ?? []} onAct={act} /> : null}
      </div>
    </section>
  );
}

type Act = (path: string, body: unknown, ok: string) => Promise<void>;

function ProviderRow({ p, can, onAct }: { p: Provider; can: boolean; onAct: Act }) {
  const [expires, setExpires] = useState("");
  const [reason, setReason] = useState("");
  return (
    <li className="rounded-lg border border-stone-200 bg-white p-4" aria-label={p.name}>
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="font-semibold">{p.name}</h3>
        <span className="text-sm font-medium" data-testid="provider-status">{p.status}</span>
      </div>
      <p className="text-sm text-stone-600">
        {p.licence_type} {p.licence_number}, {p.regulator}.{" "}
        {p.licence_verified_at ? `Verified ${p.licence_verified_at.slice(0, 10)} by ${p.licence_verified_by}.` : "Licence not yet verified."}
        {p.licence_expires_at ? ` Expires ${p.licence_expires_at.slice(0, 10)}.` : ""}
      </p>
      {can && p.status !== "WITHDRAWN" ? (
        <div className="mt-3 flex flex-wrap items-end gap-3 text-sm">
          <label>
            <span className="block font-medium">Licence expires (optional)</span>
            <input type="date" value={expires} onChange={(e) => setExpires(e.target.value)} className="mt-1 rounded-md border border-stone-300 px-3 py-2" />
          </label>
          <button
            type="button"
            className="rounded-md bg-stone-900 px-4 py-2 font-medium text-white"
            onClick={() => onAct(`/api/v1/partners/providers/${encodeURIComponent(p.provider_id)}/verify`, { licence_expires_at: expires ? `${expires}T00:00:00Z` : "" }, `${p.name}: licence verified.`)}
          >
            I checked the regulator&apos;s register: verify licence
          </button>
          <label>
            <span className="block font-medium">Reason to suspend</span>
            <input value={reason} onChange={(e) => setReason(e.target.value)} className="mt-1 w-64 rounded-md border border-stone-300 px-3 py-2" />
          </label>
          <button
            type="button"
            disabled={!reason.trim() || p.status === "SUSPENDED"}
            className="rounded-md border border-red-700 px-4 py-2 font-medium text-red-800 disabled:opacity-50"
            onClick={() => onAct(`/api/v1/partners/providers/${encodeURIComponent(p.provider_id)}/suspend`, { reason }, `${p.name} is suspended and none of its offers are shown.`)}
          >
            Suspend
          </button>
        </div>
      ) : null}
    </li>
  );
}

function RegisterForm({ onAct }: { onAct: Act }) {
  const [f, setF] = useState({ provider_id: "", name: "", licence_type: "FSP", regulator: "FSCA", licence_number: "", licence_source_url: "" });
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value });
  function submit(e: FormEvent) {
    e.preventDefault();
    void onAct("/api/v1/partners/providers", f, `${f.name} is registered. Someone else must now verify its licence.`);
  }
  const field = "mt-1 w-full rounded-md border border-stone-300 px-3 py-2";
  return (
    <form onSubmit={submit} aria-label="Register a provider" className="max-w-2xl space-y-3 rounded-lg border border-stone-200 bg-stone-50 p-4 text-sm">
      <h3 className="font-semibold">Register a provider</h3>
      <div className="grid gap-3 sm:grid-cols-2">
        <label><span className="font-medium">Short name (lower case, for the system)</span><input value={f.provider_id} onChange={set("provider_id")} className={field} /></label>
        <label><span className="font-medium">Legal name</span><input value={f.name} onChange={set("name")} className={field} /></label>
        <label>
          <span className="font-medium">Licence type</span>
          <select value={f.licence_type} onChange={set("licence_type")} className={field}>
            <option value="FSP">FSP: financial services provider (investments)</option>
            <option value="CASP">CASP: crypto asset service provider</option>
          </select>
        </label>
        <label><span className="font-medium">Regulator</span><input value={f.regulator} onChange={set("regulator")} className={field} /></label>
        <label><span className="font-medium">Licence number</span><input value={f.licence_number} onChange={set("licence_number")} className={field} /></label>
        <label><span className="font-medium">Link to the regulator&apos;s register entry (https)</span><input value={f.licence_source_url} onChange={set("licence_source_url")} className={field} /></label>
      </div>
      <button type="submit" className="rounded-md bg-stone-900 px-4 py-2 font-medium text-white">Register provider</button>
    </form>
  );
}

function OfferRow({ o, can, onAct }: { o: AdminOffer; can: boolean; onAct: Act }) {
  const id = encodeURIComponent(o.offer.offer_id);
  return (
    <li className="rounded-lg border border-stone-200 bg-white p-4" aria-label={o.offer.title}>
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="font-semibold">{o.offer.title}</h3>
        <span className="text-sm">
          <span data-testid="offer-status" className="font-medium">{o.status}</span>
          {o.visible_now ? " · customers can see it now" : " · hidden from customers"}
        </span>
      </div>
      <p className="text-sm text-stone-600">{o.offer.provider_name} · {o.offer.category} · last written by {o.updated_by || "unknown"}</p>
      <details className="mt-2 text-sm">
        <summary className="cursor-pointer font-medium">The statement customers will be shown</summary>
        <p className="mt-2 whitespace-pre-line rounded-md border border-stone-300 bg-stone-50 p-3">{o.offer.statement}</p>
      </details>
      {can ? (
        <div className="mt-3 flex gap-2 text-sm">
          {o.status !== "ACTIVE" ? (
            <button type="button" className="rounded-md bg-stone-900 px-4 py-2 font-medium text-white" onClick={() => onAct(`/api/v1/partners/offers/${id}/status`, { status: "ACTIVE" }, `${o.offer.title} is switched on.`)}>
              I read the statement: switch on
            </button>
          ) : null}
          {o.status !== "WITHDRAWN" ? (
            <button type="button" className="rounded-md border border-stone-400 px-4 py-2 font-medium" onClick={() => onAct(`/api/v1/partners/offers/${id}/status`, { status: "WITHDRAWN" }, `${o.offer.title} is withdrawn.`)}>
              Withdraw
            </button>
          ) : null}
        </div>
      ) : null}
    </li>
  );
}

function OfferForm({ providers, onAct }: { providers: Provider[]; onAct: Act }) {
  const [f, setF] = useState({ offer_id: "", provider_id: "", category: "INVESTMENTS", title: "", summary: "", handoff_url: "", fee_disclosure: "" });
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value });
  function submit(e: FormEvent) {
    e.preventDefault();
    void onAct("/api/v1/partners/offers", { ...f, provider_id: f.provider_id || providers[0]?.provider_id || "" }, `${f.title} is saved as a draft. Someone else must read its statement and switch it on.`);
  }
  const field = "mt-1 w-full rounded-md border border-stone-300 px-3 py-2";
  return (
    <form onSubmit={submit} aria-label="Write an offer" className="max-w-2xl space-y-3 rounded-lg border border-stone-200 bg-stone-50 p-4 text-sm">
      <h3 className="font-semibold">Write an offer</h3>
      <div className="grid gap-3 sm:grid-cols-2">
        <label><span className="font-medium">Short name (lower case)</span><input value={f.offer_id} onChange={set("offer_id")} className={field} /></label>
        <label>
          <span className="font-medium">Provider</span>
          <select value={f.provider_id || providers[0]?.provider_id || ""} onChange={set("provider_id")} className={field}>
            {providers.map((p) => (<option key={p.provider_id} value={p.provider_id}>{p.name} ({p.licence_type})</option>))}
          </select>
        </label>
        <label>
          <span className="font-medium">Category</span>
          <select value={f.category} onChange={set("category")} className={field}>
            <option value="INVESTMENTS">Investments (needs an FSP)</option>
            <option value="CRYPTO_ASSETS">Crypto assets (needs a CASP)</option>
          </select>
        </label>
        <label><span className="font-medium">Title</span><input value={f.title} onChange={set("title")} className={field} /></label>
      </div>
      <label className="block"><span className="font-medium">Summary</span><textarea value={f.summary} onChange={set("summary")} rows={2} className={field} /></label>
      <label className="block"><span className="font-medium">The provider&apos;s own address (https)</span><input value={f.handoff_url} onChange={set("handoff_url")} className={field} /></label>
      <label className="block"><span className="font-medium">What, if anything, Forge is paid (shown to the customer)</span><input value={f.fee_disclosure} onChange={set("fee_disclosure")} className={field} /></label>
      <button type="submit" className="rounded-md bg-stone-900 px-4 py-2 font-medium text-white">Save as draft</button>
    </form>
  );
}
