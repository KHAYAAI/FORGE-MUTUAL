import Link from "next/link";
import type { ReactNode } from "react";

// The front door's frame: a white page, near-black ink, a periwinkle accent, pill buttons and soft grey panels.
// The bank's honest status runs across the top, where a licensed bank shows its deposit-insurance line.
export function Shell({ children, wide = false }: { children: ReactNode; wide?: boolean }) {
  return (
    <div className="relative left-1/2 -mb-8 -mt-8 min-h-[calc(100vh-3.5rem)] w-screen -translate-x-1/2 bg-white text-[#1E1E1E]">
      <Notice />
      <div className={`mx-auto px-4 py-12 sm:px-6 sm:py-16 ${wide ? "max-w-6xl" : "max-w-2xl"}`}>{children}</div>
    </div>
  );
}

export function Notice() {
  return (
    <p className="bg-[#B9BCFF] px-4 py-2 text-center text-sm text-[#1E1E1E] sm:px-6">
      Forge Mutual is not yet a licensed bank. Applications open accounts in a pre-licence preview: no money is held.
    </p>
  );
}

// A grey panel with generous rounding, used for forms and choices.
export function Panel({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`rounded-[2rem] bg-[#F5F5F5] p-6 sm:p-10 ${className}`}>{children}</div>;
}

const focus = "focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#1E1E1E]";

export function ArrowLink({ href, children, solid = false }: { href: string; children: ReactNode; solid?: boolean }) {
  return (
    <Link
      href={href}
      className={`inline-flex items-center gap-2 rounded-full px-6 py-3 text-base transition-colors ${focus} ${
        solid ? "bg-[#1E1E1E] text-white hover:bg-black" : "border border-[#1E1E1E] bg-white text-[#1E1E1E] hover:bg-[#F5F5F5]"
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
      {hint ? <span className="mt-0.5 block text-xs text-[#5C5C66]">{hint}</span> : null}
      <span className="mt-2 block">{children}</span>
    </label>
  );
}

export const inputClass =
  "w-full rounded-xl border border-[#DCDCE2] bg-white px-4 py-3 text-base text-[#1E1E1E] placeholder:text-[#8E8E96] focus:border-[#5A60E8] focus:outline-none focus:ring-2 focus:ring-[#878DFF]/40";

export const buttonClass = `inline-flex items-center gap-2 rounded-full bg-[#1E1E1E] px-7 py-3 text-base text-white hover:bg-black disabled:opacity-40 ${focus}`;

// Eyebrow and headline styles: large, light, tight.
export const eyebrowClass = "text-sm font-medium text-[#5A60E8]";
export const titleClass = "mt-3 text-4xl font-light tracking-tight sm:text-5xl";
