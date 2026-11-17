import Link from "next/link";
import type { ReactNode } from "react";

// The front door's frame: warm paper, near-black ink, and the honest status of the bank across the top, where a
// licensed bank shows its deposit-insurance line.
export function Shell({ children, wide = false }: { children: ReactNode; wide?: boolean }) {
  return (
    <div className="relative left-1/2 -mb-8 -mt-8 min-h-[calc(100vh-3.5rem)] w-screen -translate-x-1/2 bg-[#F8F7F4] text-[#1A1A18]">
      <p className="bg-[#1A1A18] px-4 py-1.5 text-center text-xs italic text-[#F8F7F4] sm:text-left sm:px-6">
        Forge Mutual is not yet a licensed bank. Applications open accounts in a pre-licence preview: no money is held.
      </p>
      <div className={`mx-auto px-4 py-10 sm:px-6 sm:py-14 ${wide ? "max-w-6xl" : "max-w-xl"}`}>{children}</div>
    </div>
  );
}

export function ArrowLink({ href, children, solid = false }: { href: string; children: ReactNode; solid?: boolean }) {
  return (
    <Link
      href={href}
      className={`inline-flex items-center gap-2 rounded-md px-4 py-2 text-sm font-medium transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#1A1A18] ${
        solid ? "bg-[#1A1A18] text-[#F8F7F4] hover:bg-black" : "bg-[#EFECE5] text-[#1A1A18] hover:bg-[#E4E0D6]"
      }`}
    >
      {children}
      <span aria-hidden="true">→</span>
    </Link>
  );
}

export function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="text-sm font-medium">{label}</span>
      {hint ? <span className="mt-0.5 block text-xs text-[#6B6860]">{hint}</span> : null}
      <span className="mt-2 block">{children}</span>
    </label>
  );
}

export const inputClass =
  "w-full rounded-md border border-[#D9D4C7] bg-white px-3 py-2.5 text-sm text-[#1A1A18] placeholder:text-[#9A968B] focus:border-[#1A1A18] focus:outline-none focus:ring-1 focus:ring-[#1A1A18]";

export const buttonClass =
  "inline-flex items-center gap-2 rounded-md bg-[#1A1A18] px-5 py-2.5 text-sm font-medium text-[#F8F7F4] hover:bg-black disabled:opacity-50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#1A1A18]";
