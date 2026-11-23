import type { Metadata } from "next";
import { ArrowLink, Notice } from "@/components/apply/Shell";

export const metadata: Metadata = {
  title: "Open an account · Forge",
  description: "Apply for a Forge account for yourself or your business.",
};

// The front door: a periwinkle sky with glass discs drifting across it, a large light headline, and two ways in.
export default function ApplyLanding() {
  return (
    <div className="relative left-1/2 -mt-8 w-screen -translate-x-1/2 bg-white text-[#1E1E1E]">
      <Notice />

      <section className="px-3 pt-3 sm:px-6 sm:pt-6" aria-labelledby="apply-title">
        <div className="relative isolate overflow-hidden rounded-[2rem] bg-gradient-to-br from-[#7C82FF] via-[#9A9EFF] to-[#C7CAFF]">
          <Discs />
          <div className="relative mx-auto flex min-h-[72vh] max-w-6xl flex-col justify-end px-6 pb-14 pt-24 sm:px-10 sm:pb-20">
            <h1 id="apply-title" className="max-w-3xl text-5xl font-light leading-[1.05] tracking-tight text-[#1E1E1E] sm:text-7xl lg:text-8xl">
              Banking for the people who build.
            </h1>
            <p className="mt-6 max-w-lg text-lg text-[#1E1E1E]">
              One account for you, or one for your company. Apply in a few minutes, verify who you are, and we will tell you
              as soon as it is open.
            </p>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-6xl px-4 pb-6 pt-20 sm:px-6" aria-label="Choose an account">
        <h2 className="max-w-2xl text-4xl font-light tracking-tight sm:text-5xl">Choose your account.</h2>
        <div className="mt-10 grid gap-4 sm:grid-cols-2">
          <Choice title="Personal account" text="For you. Your South African ID and a selfie are all it takes." href="/apply/personal" />
          <Choice
            title="Business account"
            text="For a registered company. We verify the company and each director."
            href="/apply/business"
          />
        </div>
      </section>

      <footer className="mx-auto flex max-w-6xl flex-wrap justify-between gap-2 px-4 py-12 text-sm text-[#5C5C66] sm:px-6">
        <span>© 2026 Forge Mutual</span>
        <span>
          Already a customer?{" "}
          <a className="underline hover:text-[#1E1E1E]" href="/sign-in">
            Sign in
          </a>
        </span>
      </footer>
    </div>
  );
}

function Choice({ title, text, href }: { title: string; text: string; href: string }) {
  return (
    <div className="flex min-h-64 flex-col justify-between gap-10 rounded-[2rem] bg-[#F5F5F5] p-8 sm:p-10">
      <div>
        <h3 className="text-3xl font-light tracking-tight">{title}</h3>
        <p className="mt-3 max-w-sm text-base text-[#3A3A42]">{text}</p>
      </div>
      <div>
        <ArrowLink href={href} solid>
          Apply
        </ArrowLink>
      </div>
    </div>
  );
}

// An original illustration: translucent glass discs at different tilts, drawn as SVG so it costs no image request.
function Discs() {
  const discs = [
    { cx: 1130, cy: 170, rx: 120, ry: 34, rot: -18, o: 0.55 },
    { cx: 1290, cy: 360, rx: 150, ry: 150, rot: 0, o: 0.35 },
    { cx: 930, cy: 300, rx: 70, ry: 170, rot: 24, o: 0.45 },
    { cx: 1200, cy: 600, rx: 95, ry: 30, rot: 12, o: 0.6 },
    { cx: 760, cy: 120, rx: 55, ry: 55, rot: 0, o: 0.3 },
  ];
  return (
    <svg aria-hidden="true" className="absolute inset-0 -z-10 h-full w-full" viewBox="0 0 1440 800" preserveAspectRatio="xMaxYMid slice">
      <defs>
        <linearGradient id="glass" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#FFFFFF" stopOpacity="0.95" />
          <stop offset="0.45" stopColor="#B4B8FF" stopOpacity="0.6" />
          <stop offset="1" stopColor="#3D44D6" stopOpacity="0.85" />
        </linearGradient>
      </defs>
      {discs.map((d, i) => (
        <g key={i} transform={`rotate(${d.rot} ${d.cx} ${d.cy})`} opacity={d.o + 0.3}>
          <ellipse cx={d.cx + 6} cy={d.cy + 8} rx={d.rx} ry={d.ry} fill="#3D44D6" opacity="0.35" />
          <ellipse cx={d.cx} cy={d.cy} rx={d.rx} ry={d.ry} fill="url(#glass)" stroke="#FFFFFF" strokeOpacity="0.8" strokeWidth="2" />
        </g>
      ))}
    </svg>
  );
}
