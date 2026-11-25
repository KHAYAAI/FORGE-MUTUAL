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

  const [menu, setMenu] = useState(false);
  useEffect(() => setMenu(false), [pathname]);
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

  const links = who?.signedIn ? (who.customer ? [...LINKS, ...OPTIONAL.filter((o) => on[o.href])] : [{ href: "/apply", label: "Apply" }]) : [];

  if (!who?.signedIn) {
    return (
      <header className="border-b border-stone-200 bg-white/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-4 sm:px-6">
          <Link href="/accounts" className="flex items-center gap-2 text-lg font-semibold tracking-tight">
            <Mark /> Forge
          </Link>
          <div className="flex items-center gap-2 text-sm">
            <Link href="/sign-in" className="rounded-lg px-4 py-2 text-stone-700 hover:bg-stone-100">
              Sign in
            </Link>
            <Link href="/apply" className="rounded-lg bg-stone-950 px-4 py-2 font-medium text-white hover:bg-black">
              Open account
            </Link>
          </div>
        </div>
      </header>
    );
  }

  const linkList = (
    <nav aria-label="Main" className="flex flex-col gap-1 px-3 text-sm lg:flex-1">
      {links.map((l) => {
        const active = pathname.startsWith(l.href);
        return (
          <Link
            key={l.href}
            href={l.href}
            aria-current={active ? "page" : undefined}
            className={`flex items-center gap-3 rounded-lg px-3 py-2.5 lg:py-2 ${active ? "bg-stone-100 font-medium text-stone-950" : "text-stone-600 hover:bg-stone-50 hover:text-stone-950"}`}
          >
            <Icon name={l.label} />
            {l.label}
          </Link>
        );
      })}
    </nav>
  );
  const footer = (
    <div className="flex flex-col gap-1 border-t border-stone-200 px-3 py-3 text-sm">
      {who.customer ? (
        <a href="/api/v1/my-data" className="rounded-lg px-3 py-2 text-stone-500 hover:bg-stone-50" title="Download what Forge holds about you">
          My data
        </a>
      ) : null}
      <div className="flex items-center gap-3 px-3 py-2">
        <span aria-hidden className="grid h-8 w-8 place-items-center rounded-full bg-sky-50 text-xs font-semibold text-sky-700">
          {who.actor.slice(0, 1).toUpperCase()}
        </span>
        <span className="truncate text-stone-700" data-testid="signed-in-as">
          {who.actor}
        </span>
      </div>
      <button type="button" onClick={signOut} className="rounded-lg border border-stone-200 px-3 py-2 text-stone-700 hover:bg-stone-50">
        Sign out
      </button>
    </div>
  );

  return (
    <>
      {/* Phones and tablets: a slim top bar with a menu. */}
      <header className="sticky top-0 z-30 flex items-center justify-between border-b border-stone-200 bg-white/90 px-4 py-3 backdrop-blur lg:hidden">
        <Link href="/accounts" className="flex items-center gap-2 text-lg font-semibold tracking-tight">
          <Mark /> Forge
        </Link>
        <button
          type="button"
          onClick={() => setMenu((m) => !m)}
          aria-expanded={menu}
          aria-controls="mobile-menu"
          className="grid h-10 w-10 place-items-center rounded-lg border border-stone-200 text-stone-700"
        >
          <span className="sr-only">{menu ? "Close menu" : "Open menu"}</span>
          <svg aria-hidden viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <path d={menu ? "M6 6l12 12M18 6L6 18" : "M4 7h16M4 12h16M4 17h16"} />
          </svg>
        </button>
      </header>
      {menu ? (
        <div className="fixed inset-0 z-40 lg:hidden">
          <button type="button" aria-label="Close menu" onClick={() => setMenu(false)} className="absolute inset-0 bg-stone-950/30" />
          <div id="mobile-menu" className="absolute top-0 bottom-0 left-0 flex w-72 max-w-[85vw] flex-col overflow-y-auto bg-white pt-4 shadow-xl">
            <div className="px-5 pb-4">
              <Link href="/accounts" className="flex items-center gap-2 text-lg font-semibold tracking-tight">
                <Mark /> Forge
              </Link>
            </div>
            <div className="flex-1">{linkList}</div>
            {footer}
          </div>
        </div>
      ) : null}

      {/* Laptops and desktops: a fixed sidebar. */}
      <aside className="hidden border-r border-stone-200 bg-white lg:sticky lg:top-0 lg:flex lg:h-screen lg:w-60 lg:shrink-0 lg:flex-col xl:w-64">
        <div className="px-5 py-6">
          <Link href="/accounts" className="flex items-center gap-2 text-lg font-semibold tracking-tight">
            <Mark /> Forge
          </Link>
        </div>
        {linkList}
        {footer}
      </aside>
    </>
  );
}

function Mark() {
  return <span aria-hidden className="grid h-7 w-7 place-items-center rounded-lg bg-stone-950 text-xs font-bold text-white">F</span>;
}

const PATHS: Record<string, string> = {
  Accounts: "M3 10h18M5 6h14a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2z",
  Pay: "M5 12h14M13 6l6 6-6 6",
  FX: "M7 7h12l-3-3M17 17H5l3 3",
  Currencies: "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18",
  Savings: "M4 19h16M6 16V9M10 16V9M14 16V9M18 16V9M3 8l9-5 9 5",
  Credit: "M12 3v18M17 7H9.5a3 3 0 0 0 0 6h5a3 3 0 0 1 0 6H6",
  "Get paid": "M12 5v14M6 13l6 6 6-6",
  Payouts: "M4 7h16M4 12h16M4 17h10",
  Cards: "M3 7h18v10H3zM3 11h18",
  Invest: "M4 18l5-6 4 3 7-9",
  Apply: "M9 12l2 2 4-4M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z",
};

function Icon({ name }: { name: string }) {
  return (
    <svg aria-hidden viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d={PATHS[name] ?? PATHS.Accounts} />
    </svg>
  );
}
