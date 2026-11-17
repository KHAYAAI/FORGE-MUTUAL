"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ArrowLink, Shell } from "@/components/apply/Shell";

// The step from approved applicant to customer.
export default function Welcome() {
  const router = useRouter();
  const [message, setMessage] = useState<string | null>(null);
  useEffect(() => {
    fetch("/api/session/attach", { method: "POST" })
      .then(async (r) => {
        if (r.ok) {
          router.replace("/accounts");
          router.refresh();
          return;
        }
        const b = (await r.json().catch(() => null)) as { error?: { message?: string } } | null;
        setMessage(b?.error?.message ?? "We could not open your account view. Try again shortly.");
      })
      .catch(() => setMessage("We could not reach Forge. Try again shortly."));
  }, [router]);
  return (
    <Shell>
      <h1 className="text-3xl font-normal tracking-tight">Welcome to Forge</h1>
      {message ? (
        <div className="mt-6 space-y-4">
          <p className="text-sm text-[#3B3A36]">{message}</p>
          <ArrowLink href="/sign-in?next=/accounts" solid>
            Sign in
          </ArrowLink>
        </div>
      ) : (
        <p className="mt-4 text-sm text-[#6B6860]">Opening your account…</p>
      )}
    </Shell>
  );
}
