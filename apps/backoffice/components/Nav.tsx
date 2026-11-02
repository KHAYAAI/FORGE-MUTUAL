"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useStaff } from "@/lib/useStaff";

export default function Nav() {
  const router = useRouter();
  const pathname = usePathname();
  const staff = useStaff(pathname);

  async function signOut() {
    await fetch("/api/session", { method: "DELETE" });
    router.push("/sign-in");
    router.refresh();
  }

  return (
    <header className="border-b border-stone-300 bg-white">
      <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 px-4 py-3">
        <Link href="/cases" className="text-lg font-semibold tracking-tight">
          Forge back office
        </Link>
        {staff?.signedIn ? (
          <nav aria-label="Main" className="flex flex-wrap items-center gap-4 text-sm">
            <Link href="/cases" className="text-stone-700 hover:text-stone-950">
              Cases
            </Link>
            <Link href="/recon" className="text-stone-700 hover:text-stone-950">
              Reconciliation
            </Link>
            <Link href="/payments/lookup" className="text-stone-700 hover:text-stone-950">
              Payments
            </Link>
            <Link href="/kyc" className="text-stone-700 hover:text-stone-950">
              KYC
            </Link>
            <span className="text-stone-700" data-testid="staff-name">
              {staff.name}
            </span>
            <span className="text-xs text-stone-500" data-testid="staff-roles">
              {staff.roles.join(", ")}
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
