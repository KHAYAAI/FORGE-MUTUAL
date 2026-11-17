import type { Metadata } from "next";
import { ArrowLink } from "@/components/apply/Shell";

export const metadata: Metadata = {
  title: "Open an account · Forge",
  description: "Apply for a Forge account for yourself or your business.",
};

// The front door. A quiet landscape drawn in code, the name large and faint across it, and two ways in.
export default function ApplyLanding() {
  return (
    <div className="relative left-1/2 -mt-8 w-screen -translate-x-1/2 bg-[#F8F7F4] text-[#1A1A18]">
      <p className="bg-[#1A1A18] px-4 py-1.5 text-xs italic text-[#F8F7F4] sm:px-6">
        Forge Mutual is not yet a licensed bank. Applications open accounts in a pre-licence preview: no money is held.
      </p>

      <section className="relative isolate overflow-hidden" aria-labelledby="apply-title">
        <Landscape />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-x-0 -bottom-[0.12em] select-none text-center font-light leading-none tracking-tighter text-white/35"
          style={{ fontSize: "clamp(6rem, 24vw, 22rem)" }}
        >
          FORGE
        </div>
        <div className="relative mx-auto flex min-h-[78vh] max-w-6xl flex-col justify-start px-4 pb-64 pt-16 sm:px-6 sm:pt-24">
          <h1 id="apply-title" className="max-w-xl text-4xl font-normal tracking-tight sm:text-5xl">
            Banking for the people who build.
          </h1>
          <p className="mt-5 max-w-md text-base text-[#3B3A36]">
            One account for you, or one for your company. Apply in a few minutes, verify who you are, and we will tell you
            as soon as it is open.
          </p>
        </div>
      </section>

      <section className="mx-auto grid max-w-6xl gap-4 px-4 py-14 sm:grid-cols-2 sm:px-6" aria-label="Choose an account">
        <Choice
          title="Personal account"
          text="For you. Your South African ID and a selfie are all it takes."
          href="/apply/personal"
        />
        <Choice
          title="Business account"
          text="For a registered company. We verify the company and each director."
          href="/apply/business"
        />
      </section>

      <footer className="mx-auto flex max-w-6xl flex-wrap justify-between gap-2 px-4 pb-10 text-xs text-[#6B6860] sm:px-6">
        <span>© 2026 Forge Mutual</span>
        <span>
          Already a customer? <a className="underline hover:text-[#1A1A18]" href="/sign-in">Sign in</a>
        </span>
      </footer>
    </div>
  );
}

function Choice({ title, text, href }: { title: string; text: string; href: string }) {
  return (
    <div className="flex flex-col justify-between gap-8 rounded-lg bg-[#EFECE5] p-6 sm:p-8">
      <div>
        <h2 className="text-xl font-normal tracking-tight">{title}</h2>
        <p className="mt-2 text-sm text-[#4A4842]">{text}</p>
      </div>
      <div>
        <ArrowLink href={href} solid>
          Apply
        </ArrowLink>
      </div>
    </div>
  );
}

// An original illustration: terraced hills, water and fields, in the palette of the page. Drawn as SVG so it costs
// no image request and needs no licence.
function Landscape() {
  return (
    <svg
      aria-hidden="true"
      className="absolute inset-0 -z-10 h-full w-full"
      viewBox="0 0 1440 800"
      preserveAspectRatio="xMidYMid slice"
    >
      <defs>
        <linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#E9EEEA" />
          <stop offset="1" stopColor="#F4F1EA" />
        </linearGradient>
        <linearGradient id="water" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#8FB9B8" />
          <stop offset="1" stopColor="#5E918F" />
        </linearGradient>
      </defs>
      <rect width="1440" height="800" fill="url(#sky)" />
      <path d="M0 470 C 240 400 420 430 640 380 S 1040 330 1440 360 L1440 800 L0 800 Z" fill="#C9D3B8" />
      <path d="M0 540 C 260 480 520 520 760 470 S 1180 430 1440 460 L1440 800 L0 800 Z" fill="#A9BC93" />
      <path d="M0 620 C 300 560 560 610 840 560 S 1220 540 1440 570 L1440 800 L0 800 Z" fill="#7F9C6E" />
      <path d="M-20 700 C 260 640 520 690 760 650 S 1180 640 1460 680 L1460 820 L-20 820 Z" fill="url(#water)" />
      <g fill="none" stroke="#F8F7F4" strokeOpacity="0.5" strokeWidth="1.2">
        <path d="M0 505 C 260 450 470 480 700 430 S 1100 395 1440 410" />
        <path d="M0 580 C 280 520 540 565 800 515 S 1200 490 1440 515" />
        <path d="M0 655 C 300 600 560 650 820 605 S 1220 595 1440 625" />
      </g>
      <g fill="#E8DFC9" stroke="#B9AE95" strokeWidth="1">
        <rect x="1030" y="300" width="22" height="64" />
        <rect x="1058" y="282" width="22" height="82" />
        <rect x="1086" y="318" width="22" height="46" />
      </g>
      <g stroke="#F8F7F4" strokeWidth="2" strokeLinecap="round">
        <line x1="1250" y1="250" x2="1250" y2="345" />
        <line x1="1250" y1="250" x2="1232" y2="232" />
        <line x1="1250" y1="250" x2="1274" y2="244" />
        <line x1="1250" y1="250" x2="1246" y2="276" />
        <line x1="1330" y1="270" x2="1330" y2="352" />
        <line x1="1330" y1="270" x2="1314" y2="254" />
        <line x1="1330" y1="270" x2="1351" y2="265" />
        <line x1="1330" y1="270" x2="1327" y2="293" />
      </g>
    </svg>
  );
}
