"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

type Who = { signedIn: true; actor: string; customer: boolean } | { signedIn: false };

const LINKS = [
  { href: "/accounts", label: "Accounts" },
  { href: "/payments/new", label: "Pay" },
  { href: "/fx", label: "FX" },
  { href: "/currencies", label: "Currencies" },
  { href: "/savings", label: "Savings" },
  { href: "/credit", label: "Credit" },
  { href: "/get-paid", label: "Get paid" },
  { href: "/payouts/new", label: "Payouts" },
];

// Products the bank can switch on or off. A link to one appears only while its route answers 200, so a product that is off is not advertised.
const OPTIONAL = [
  { href: "/cards", label: "Cards", probe: "/api/v1/cards" },
  { href: "/invest", label: "Invest", probe: "/api/v1/offers" },
];

export default function Nav() {
  const router = useRouter();
  const pathname = usePathname();
  const [who, setWho] = useState<Who | null>(null);

  useEffect(() => {
    let live = true;
    fetch("/api/session", { cache: "no-store" })
      .then(async (res) => {
        const body = (await res.json()) as { signedIn: boolean; actor?: string; party?: string };
        if (live)
          setWho(body.signedIn && body.actor ? { signedIn: true, actor: body.actor, customer: !!body.party } : { signedIn: false });
      })
      .catch(() => live && setWho({ signedIn: false }));
    return () => {
      live = false;
    };
  }, [pathname]);

  const [on, setOn] = useState<Record<string, boolean>>({});
  const isCustomer = who?.signedIn === true && who.customer;
  useEffect(() => {
    if (!isCustomer) return setOn({});
    let live = true;
    Promise.all(OPTIONAL.map((o) => fetch(o.probe, { cache: "no-store" }).then((r) => [o.href, r.ok] as const).catch(() => [o.href, false] as const))).then(
      (r) => live && setOn(Object.fromEntries(r)),
    );
    return () => {
      live = false;
    };
  }, [isCustomer, pathname]);

  async function signOut() {
    await fetch("/api/session", { method: "DELETE" });
    setWho({ signedIn: false });
    router.push("/sign-in");
  }

  return (
    <header className="bg-white">
      <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 px-4 py-4 sm:px-6">
        <Link href="/accounts" className="text-2xl font-light tracking-tight">
          Forge
        </Link>
        {who?.signedIn ? (
          <nav aria-label="Main" className="flex flex-wrap items-center gap-1 text-base">
            {(who.customer ? [...LINKS, ...OPTIONAL.filter((o) => on[o.href])] : [{ href: "/apply", label: "Apply" }]).map((l) => (
              <Link
                key={l.href}
                href={l.href}
                aria-current={pathname.startsWith(l.href) ? "page" : undefined}
                className={`rounded-full px-4 py-2 ${pathname.startsWith(l.href) ? "bg-stone-100 text-stone-950" : "text-stone-700 hover:bg-stone-100"}`}
              >
                {l.label}
              </Link>
            ))}
            {who.customer ? (
              <a href="/api/v1/my-data" className="rounded-full px-3 py-2 text-sm text-stone-500 hover:bg-stone-100" title="Download what Forge holds about you">
                My data
              </a>
            ) : null}
            <span className="px-3 text-stone-500" data-testid="signed-in-as">
              {who.actor}
            </span>
            <button type="button" onClick={signOut} className="rounded-full border border-stone-900 px-4 py-2 text-stone-900 hover:bg-stone-100">
              Sign out
            </button>
          </nav>
        ) : (
          <div className="flex items-center gap-2 text-base">
            <Link href="/sign-in" className="rounded-full px-4 py-2 text-stone-700 hover:bg-stone-100">
              Sign in
            </Link>
            <Link href="/apply" className="rounded-full bg-stone-900 px-5 py-2 text-white hover:bg-black">
              Open account
            </Link>
          </div>
        )}
      </div>
    </header>
  );
}
