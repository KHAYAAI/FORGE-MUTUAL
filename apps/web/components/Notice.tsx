import type { ReactNode } from "react";

const TONES = {
  error: "border-red-200 bg-red-50 text-red-900",
  success: "border-emerald-200 bg-emerald-50 text-emerald-900",
  info: "border-sky-200 bg-sky-50 text-sky-900",
} as const;

/** A message for the user. Errors use role="alert" so assistive technology announces them. */
export default function Notice({ tone, children }: { tone: keyof typeof TONES; children: ReactNode }) {
  return (
    <div
      role={tone === "error" ? "alert" : "status"}
      data-testid="notice"
      className={`rounded-md border px-4 py-3 text-sm ${TONES[tone]}`}
    >
      {children}
    </div>
  );
}
