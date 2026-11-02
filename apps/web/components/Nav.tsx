"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

type Who = { signedIn: true; actor: string } | { signedIn: false };

const LINKS = [
  { href: "/accounts", label: "Accounts" },
  { href: "/payments/new", label: "Pay" },
  { href: "/fx", label: "FX" },
  { href: "/payouts/new", label: "Payouts" },
];

export default function Nav() {
  const router = useRouter();
  const pathname = usePathname();
  const [who, setWho] = useState<Who | null>(null);

  useEffect(() => {
    let live = true;
    fetch("/api/session", { cache: "no-store" })
      .then(async (res) => {
        const body = (await res.json()) as { signedIn: boolean; actor?: string };
        if (live) setWho(body.signedIn && body.actor ? { signedIn: true, actor: body.actor } : { signedIn: false });
      })
      .catch(() => live && setWho({ signedIn: false }));
    return () => {
      live = false;
    };
  }, [pathname]);

  async function signOut() {
    await fetch("/api/session", { method: "DELETE" });
    setWho({ signedIn: false });
    router.push("/sign-in");
  }

  return (
    <header className="border-b border-stone-200 bg-white">
      <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-4 px-4 py-3">
        <Link href="/accounts" className="text-lg font-semibold tracking-tight">
          Forge
        </Link>
        {who?.signedIn ? (
          <nav aria-label="Main" className="flex flex-wrap items-center gap-4 text-sm">
            {LINKS.map((l) => (
              <Link key={l.href} href={l.href} className="text-stone-700 hover:text-stone-950">
                {l.label}
              </Link>
            ))}
            <span className="text-stone-500" data-testid="signed-in-as">
              {who.actor}
            </span>
            <button type="button" onClick={signOut} className="text-stone-600 underline hover:text-stone-900">
              Sign out
            </button>
          </nav>
        ) : (
          <Link href="/sign-in" className="text-sm text-stone-700 hover:text-stone-950">
            Sign in
          </Link>
        )}
      </div>
    </header>
  );
}
