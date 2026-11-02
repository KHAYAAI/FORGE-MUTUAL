"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { errorText } from "@/lib/api/client";

type Mode = "oidc" | "dev" | null;

export default function SignInPage() {
  const router = useRouter();
  const [mode, setMode] = useState<Mode>(null);
  const [callbackError, setCallbackError] = useState<string | null>(null);
  const [party, setParty] = useState("");
  const [actor, setActor] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setCallbackError(new URLSearchParams(window.location.search).get("error"));
    fetch("/api/auth/mode", { cache: "no-store" })
      .then((r) => r.json())
      .then((b: { mode: Mode }) => setMode(b.mode))
      .catch(() => setError("We could not reach the sign-in service. Try again shortly."));
  }, []);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const res = await fetch("/api/session", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ party: party.trim(), actor: actor.trim() }),
    });
    setBusy(false);
    if (!res.ok) {
      const body = await res.json().catch(() => null);
      return setError(errorText(body, "We could not sign you in. Check the details and try again."));
    }
    router.push("/accounts");
    router.refresh();
  }

  if (mode === "oidc") {
    return (
      <section className="mx-auto max-w-md space-y-6">
        <h1 className="text-2xl font-semibold">Sign in</h1>
        {callbackError ? <Notice tone="error">{callbackError}</Notice> : null}
        <a
          href="/api/auth/login"
          className="inline-block rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white hover:bg-stone-800"
        >
          Sign in with your identity provider
        </a>
      </section>
    );
  }

  return (
    <section className="mx-auto max-w-md space-y-6">
      <h1 className="text-2xl font-semibold">Sign in</h1>
      {mode === "dev" ? (
        <Notice tone="info">Development sign-in: enter the party's ID and your name. It is not available in production.</Notice>
      ) : null}
      {error ? <Notice tone="error">{error}</Notice> : null}
      {mode === "dev" ? (
        <form onSubmit={submit} className="space-y-4" aria-label="Sign in">
          <label className="block text-sm">
            <span className="font-medium">Party ID</span>
            <input
              name="party"
              value={party}
              onChange={(e) => setParty(e.target.value)}
              required
              autoComplete="off"
              className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2 font-mono text-sm"
            />
          </label>
          <label className="block text-sm">
            <span className="font-medium">Your name</span>
            <input
              name="actor"
              value={actor}
              onChange={(e) => setActor(e.target.value)}
              required
              maxLength={64}
              autoComplete="name"
              className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2 text-sm"
            />
          </label>
          <button
            type="submit"
            disabled={busy}
            className="rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white hover:bg-stone-800 disabled:opacity-50"
          >
            {busy ? "Signing in…" : "Sign in"}
          </button>
        </form>
      ) : null}
    </section>
  );
}
