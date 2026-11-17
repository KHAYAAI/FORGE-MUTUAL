"use client";

import { type ReactNode, useEffect, useState } from "react";
import { ArrowLink } from "@/components/apply/Shell";

// Shows its children only to someone signed in. Applying needs a verified sign-in, so an application belongs to a
// person the identity provider knows, and they can find it again later.
export function RequireApplicant({ next, children }: { next: string; children: ReactNode }) {
  const [state, setState] = useState<"checking" | "in" | "out">("checking");
  useEffect(() => {
    fetch("/api/session", { cache: "no-store" })
      .then((r) => r.json())
      .then((b: { signedIn?: boolean; applicant?: boolean }) => setState(b.signedIn && b.applicant ? "in" : "out"))
      .catch(() => setState("out"));
  }, []);
  if (state === "checking") return <p className="text-sm text-[#6B6860]">Checking that you are signed in…</p>;
  if (state === "out") {
    return (
      <div className="space-y-5">
        <p className="text-sm text-[#3B3A36]">
          Sign in first. It keeps your application safe, and lets you come back to it on any device.
        </p>
        <ArrowLink href={`/sign-in?next=${encodeURIComponent(next)}`} solid>
          Sign in to apply
        </ArrowLink>
      </div>
    );
  }
  return <>{children}</>;
}
