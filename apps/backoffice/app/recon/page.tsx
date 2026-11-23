"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor } from "@/lib/money";
import { hasRole, useStaff } from "@/lib/useStaff";

type Gate = components["schemas"]["Gate"];
type Break = components["schemas"]["Break"];

export default function ReconPage() {
  const router = useRouter();
  const staff = useStaff("/recon");
  const [gate, setGate] = useState<Gate | null>(null);
  const [breaks, setBreaks] = useState<Break[] | null>(null);
  const [openOnly, setOpenOnly] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    (async () => {
      const g = await api.GET("/v1/recon/gate");
      if (!live) return;
      if (g.response.status === 401) return router.push("/sign-in");
      if (g.error) return setError(errorText(g.error, "The gate could not be read."));
      if (!g.data) return setError("The gate could not be read.");
      const b = await api.GET("/v1/recon/breaks", { params: { query: { open_only: openOnly } } });
      if (!live) return;
      if (b.response.status === 401) return router.push("/sign-in");
      if (b.error) return setError(errorText(b.error, "The breaks could not be read."));
      if (!b.data) return setError("The breaks could not be read.");
      setError(null);
      setGate(g.data);
      setBreaks(b.data.breaks);
    })();
    return () => {
      live = false;
    };
  }, [openOnly, router]);

  return (
    <section aria-labelledby="recon-heading" className="space-y-6">
      <h1 id="recon-heading" className="text-2xl font-semibold">
        Reconciliation
      </h1>
      {error ? <Notice tone="error">{error}</Notice> : null}

      {gate ? (
        gate.break_active ? (
          <div role="alert" data-testid="gate" className="rounded-md border border-red-300 bg-red-50 p-4 text-sm text-red-900">
            <p className="font-semibold">The payments gate is closed. Payments are held.</p>
            <ul className="mt-2 list-disc pl-5">
              {gate.reasons.map((r) => (
                <li key={r}>{r}</li>
              ))}
            </ul>
          </div>
        ) : (
          <Notice tone="success">The payments gate is open.</Notice>
        )
      ) : null}

      <div className="flex items-center justify-between">
        <h2 className="text-lg font-medium">Breaks</h2>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={!openOnly} onChange={(e) => setOpenOnly(!e.target.checked)} />
          Include cleared breaks
        </label>
      </div>
      <p className="text-sm text-stone-600">
        An investigation records what was found. It does not clear a break. Only a later run that matches can clear it.
      </p>

      {breaks?.length === 0 ? <p className="text-stone-600">No breaks to show.</p> : null}
      {breaks?.map((b) => (
        <BreakRow key={b.break_id} brk={b} canInvestigate={hasRole(staff ?? null, "operations")} onSaved={() => setOpenOnly((v) => v)} />
      ))}
    </section>
  );
}

function BreakRow({ brk, canInvestigate, onSaved }: { brk: Break; canInvestigate: boolean; onSaved: () => void }) {
  const [current, setCurrent] = useState(brk);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function investigate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    const { data, error } = await api.POST("/v1/recon/breaks/{break}/investigate", {
      params: { path: { break: current.break_id } },
      body: { note: note.trim() },
    });
    setBusy(false);
    if (error || !data) return setError(errorText(error, "The investigation was not recorded. Try again."));
    setCurrent(data);
    setNote("");
    onSaved();
  }

  return (
    <article className="space-y-3 rounded-lg bg-white p-5 text-sm shadow-sm" aria-label={`Break ${current.break_id.slice(0, 8)}`}>
      <div className="flex flex-wrap items-center gap-3">
        <span className="font-mono">{current.break_id.slice(0, 8)}</span>
        <span>{current.asset}</span>
        <span>{current.check}</span>
        <span className="tabular-nums">{formatMinor(current.difference_minor ?? "0", current.asset ?? "ZAR")}</span>
        <span
          data-testid="break-status"
          className={current.status === "OPEN" ? "rounded bg-red-100 px-2 py-0.5 text-red-900" : "rounded bg-emerald-100 px-2 py-0.5 text-emerald-900"}
        >
          {current.status === "OPEN" ? "Open" : "Cleared"}
        </span>
      </div>
      <p className="text-stone-700">{current.detail}</p>
      {current.investigation_note ? (
        <p className="rounded bg-stone-100 p-3 text-stone-800">
          <span className="font-medium">Investigation by {current.investigated_by}:</span> {current.investigation_note}
        </p>
      ) : null}
      {error ? <Notice tone="error">{error}</Notice> : null}
      {current.status === "OPEN" && canInvestigate ? (
        <form onSubmit={investigate} className="space-y-2" aria-label="Record an investigation">
          <label className="block">
            <span className="font-medium">What did you find?</span>
            <textarea
              name="note"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              rows={2}
              required
              className="mt-1 w-full rounded-md border border-stone-300 px-3 py-2"
            />
          </label>
          <button
            type="submit"
            disabled={busy || note.trim() === ""}
            className="rounded-md bg-stone-900 px-3 py-1.5 font-medium text-white hover:bg-stone-800 disabled:opacity-50"
          >
            Record investigation
          </button>
        </form>
      ) : null}
    </article>
  );
}
