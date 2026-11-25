// Decorative pictures that go with the screens. They are aria-hidden except where they carry text the page needs (the card number).

const CARD_SKINS = [
  "from-stone-950 via-stone-800 to-stone-700",
  "from-[#2b2f8f] via-[#4a50d6] to-[#8b8ff0]",
  "from-[#0f3d33] via-[#1f6b4f] to-[#5fbf95]",
];

/** A payment card drawn to scale (85.6 × 54 mm), showing the label, last four digits and expiry. */
export function CardArt({ label, last4, expiry, frozen = false, skin = 0 }: {
  label: string;
  last4?: string;
  expiry?: string;
  frozen?: boolean;
  skin?: number;
}) {
  return (
    <div
      className={`relative aspect-[1.586] w-full max-w-sm overflow-hidden rounded-2xl bg-gradient-to-br p-5 text-white shadow-lg sm:p-6 ${CARD_SKINS[skin % CARD_SKINS.length]} ${frozen ? "grayscale" : ""}`}
    >
      <div aria-hidden className="pointer-events-none absolute -top-16 -right-10 h-48 w-48 rounded-full bg-white/10" />
      <div aria-hidden className="pointer-events-none absolute -bottom-20 -left-10 h-48 w-48 rounded-full bg-white/5" />
      <div className="relative flex h-full flex-col justify-between">
        <div className="flex items-start justify-between">
          <span className="text-sm font-semibold tracking-wide">Forge</span>
          <span className="text-xs text-white/70">Virtual debit</span>
        </div>
        <div aria-hidden className="h-7 w-10 rounded-md bg-gradient-to-br from-amber-200 to-amber-400 opacity-90" />
        <div>
          <p className="font-mono text-lg tracking-widest tabular-nums sm:text-xl">{last4 ? `•••• ${last4}` : "•••• ••••"}</p>
          <div className="mt-1 flex items-end justify-between gap-2 text-xs text-white/80">
            <span className="truncate">{label || "Your card"}</span>
            {expiry ? <span className="tabular-nums">{expiry}</span> : null}
          </div>
        </div>
      </div>
      {frozen ? (
        <div aria-hidden className="absolute inset-0 grid place-items-center bg-white/25 backdrop-blur-[2px]">
          <span className="rounded-full bg-white/90 px-3 py-1 text-xs font-semibold text-stone-900">Frozen</span>
        </div>
      ) : null}
    </div>
  );
}

const CURRENCY: Record<string, { sign: string; tone: string }> = {
  ZAR: { sign: "R", tone: "bg-emerald-100 text-emerald-800" },
  USD: { sign: "$", tone: "bg-sky-50 text-sky-700" },
  EUR: { sign: "€", tone: "bg-amber-100 text-amber-800" },
  GBP: { sign: "£", tone: "bg-rose-100 text-rose-800" },
};

/** A round currency badge. */
export function CurrencyBadge({ asset, size = "md" }: { asset: string; size?: "md" | "lg" }) {
  const c = CURRENCY[asset] ?? { sign: asset.slice(0, 1), tone: "bg-stone-100 text-stone-700" };
  return (
    <span aria-hidden className={`grid shrink-0 place-items-center rounded-full font-semibold ${c.tone} ${size === "lg" ? "h-12 w-12 text-lg" : "h-9 w-9 text-sm"}`}>
      {c.sign}
    </span>
  );
}

/** Two currencies with an exchange arrow between them. */
export function ExchangeArt({ from, to }: { from: string; to: string }) {
  return (
    <div aria-hidden className="flex items-center gap-3">
      <CurrencyBadge asset={from} size="lg" />
      <svg viewBox="0 0 48 24" className="h-6 w-12 text-stone-400" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M4 8h36l-6-5M44 16H8l6 5" />
      </svg>
      <CurrencyBadge asset={to} size="lg" />
    </div>
  );
}

/** A simple picture for a page header: a soft panel with a line icon. */
export function HeroIcon({ path }: { path: string }) {
  return (
    <span aria-hidden className="grid h-12 w-12 shrink-0 place-items-center rounded-2xl bg-sky-50 text-sky-700">
      <svg viewBox="0 0 24 24" className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <path d={path} />
      </svg>
    </span>
  );
}
