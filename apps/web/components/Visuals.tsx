// Decorative pictures that go with the screens. They are aria-hidden except where they carry text the page needs (the card number).

const CARD_SKINS = [
  { bg: "radial-gradient(120% 140% at 100% 0%, #3a3a42 0%, #16161a 55%, #0b0b0d 100%)", line: "rgb(255 255 255 / .07)", name: "Graphite" },
  { bg: "radial-gradient(120% 140% at 100% 0%, #8b8ff0 0%, #4a50d6 45%, #1f2270 100%)", line: "rgb(255 255 255 / .1)", name: "Periwinkle" },
  { bg: "radial-gradient(120% 140% at 100% 0%, #5fbf95 0%, #1f6b4f 50%, #0b2a20 100%)", line: "rgb(255 255 255 / .09)", name: "Emerald" },
];

/** A payment card drawn to scale (85.6 × 54 mm), showing the label, last four digits and expiry. */
export function CardArt({ label, last4, expiry, frozen = false, skin = 0, appear = false }: {
  label: string;
  last4?: string;
  expiry?: string;
  frozen?: boolean;
  skin?: number;
  /** Deal the card in with a short animation (a newly issued card). */
  appear?: boolean;
}) {
  const k = CARD_SKINS[Math.abs(skin) % CARD_SKINS.length];
  return (
    <div
      style={{ background: k.bg }}
      className={`card-3d relative aspect-[1.586] w-full max-w-sm overflow-hidden rounded-[18px] p-5 text-white shadow-[0_18px_40px_-20px_rgb(0_0_0/.6)] ring-1 ring-white/10 sm:p-6 ${appear ? "motion-pop" : ""} ${frozen ? "grayscale" : ""}`}
    >
      {/* Fine wave line-work, like the security print on a real card. */}
      <svg aria-hidden className="pointer-events-none absolute inset-0 h-full w-full" viewBox="0 0 320 202" preserveAspectRatio="none">
        {Array.from({ length: 14 }, (_, i) => (
          <path key={i} d={`M-10 ${40 + i * 11} C 80 ${10 + i * 11}, 180 ${90 + i * 11}, 330 ${30 + i * 11}`} fill="none" stroke={k.line} strokeWidth="1" />
        ))}
      </svg>
      {/* A holographic band along the right edge. */}
      <div
        aria-hidden
        className="pointer-events-none absolute -right-8 top-0 bottom-0 w-24 opacity-40 mix-blend-screen"
        style={{ background: "linear-gradient(160deg, transparent 10%, #ffd6f5 30%, #c9f0ff 45%, #d9ffd6 60%, transparent 80%)", filter: "blur(10px)" }}
      />
      <div aria-hidden className={`card-sheen ${appear ? "run" : ""}`} />
      <div className="relative flex h-full flex-col justify-between">
        <div className="flex items-start justify-between">
          <span className="flex items-center gap-1.5 text-[15px] font-semibold tracking-wide">
            <span aria-hidden className="grid h-5 w-5 place-items-center rounded-md bg-white text-[10px] font-bold text-stone-950">F</span>
            Forge
          </span>
          <span className="text-[11px] font-medium tracking-wider text-white/70 uppercase">Virtual debit</span>
        </div>
        <div aria-hidden className="flex items-center gap-3">
          {/* An EMV chip with its contact lines. */}
          <svg viewBox="0 0 40 30" className="h-7 w-10 drop-shadow-sm">
            <defs>
              <linearGradient id="chip" x1="0" y1="0" x2="1" y2="1">
                <stop offset="0" stopColor="#f6e3a1" />
                <stop offset=".5" stopColor="#d9b25a" />
                <stop offset="1" stopColor="#f3dc96" />
              </linearGradient>
            </defs>
            <rect x=".5" y=".5" width="39" height="29" rx="5" fill="url(#chip)" stroke="#b8913d" strokeWidth=".6" />
            <path d="M0 10h13M0 20h13M27 10h13M27 20h13M13 0v30M27 0v30M13 15h14" stroke="#a8823a" strokeWidth=".8" fill="none" />
          </svg>
          {/* Contactless symbol. */}
          <svg viewBox="0 0 24 24" className="h-5 w-5 text-white/80" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round">
            <path d="M8 8.5a5 5 0 0 1 0 7M11.5 6a9 9 0 0 1 0 12M15 3.5a13 13 0 0 1 0 17" />
          </svg>
        </div>
        <div>
          <p className="font-mono text-lg tracking-[0.2em] tabular-nums [text-shadow:0_1px_0_rgb(0_0_0/.45),0_-1px_0_rgb(255_255_255/.15)] sm:text-xl">
            {last4 ? `•••• ${last4}` : "•••• ••••"}
          </p>
          <div className="mt-2 flex items-end justify-between gap-2">
            <span className="truncate text-xs font-medium tracking-wide text-white/85 uppercase">{label || "Your card"}</span>
            <span className="flex items-end gap-3">
              {expiry ? (
                <span className="text-right leading-none">
                  <span className="block text-[8px] tracking-wider text-white/60 uppercase">Valid thru</span>
                  <span className="text-xs tabular-nums">{expiry}</span>
                </span>
              ) : null}
              {/* A generic network mark (not any real network's logo). */}
              <span aria-hidden className="flex">
                <span className="h-6 w-6 rounded-full bg-white/80" />
                <span className="-ml-2.5 h-6 w-6 rounded-full bg-white/40" />
              </span>
            </span>
          </div>
        </div>
      </div>
      {frozen ? (
        <div aria-hidden className="motion-frost absolute inset-0 grid place-items-center bg-sky-100/25 backdrop-blur-[3px]">
          <span className="flex items-center gap-1.5 rounded-full bg-white/95 px-3 py-1 text-xs font-semibold text-stone-900 shadow">
            <svg viewBox="0 0 24 24" className="h-3.5 w-3.5 text-sky-700" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <path d="M12 2v20M4.9 6.5l14.2 11M19.1 6.5 4.9 17.5M9 4l3 2 3-2M9 20l3-2 3 2" />
            </svg>
            Frozen
          </span>
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
      <span key={`f-${from}`} className="motion-pop"><CurrencyBadge asset={from} size="lg" /></span>
      <svg key={`${from}-${to}`} viewBox="0 0 48 24" className="motion-swap h-6 w-12 text-stone-400" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M4 8h36l-6-5M44 16H8l6 5" />
      </svg>
      <span key={`t-${to}`} className="motion-pop"><CurrencyBadge asset={to} size="lg" /></span>
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
