"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { type FormEvent, useCallback, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { hasRole, useStaff } from "@/lib/useStaff";

type Case = components["schemas"]["Case"];
const MIN_NOTE = 20;

export default function CaseDetailPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const caseId = decodeURIComponent(params.id);
  const staff = useStaff(`/cases/${caseId}`);
  const [kase, setKase] = useState<Case | null>(null);
  const [note, setNote] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    const { data, error, response } = await api.GET("/v1/cases/{case}", { params: { path: { case: caseId } } });
    if (response.status === 401) return router.push("/sign-in");
    if (error || !data) return setError(errorText(error, "The case could not be loaded."));
    setKase(data);
  }, [caseId, router]);

  useEffect(() => {
    void load();
  }, [load]);

  async function close(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    const { data, error, response } = await api.POST("/v1/cases/{case}/close", {
      params: { path: { case: caseId } },
      body: { note: note.trim() },
    });
    setBusy(false);
    if (response.status === 401) return router.push("/sign-in");
    if (error || !data) return setError(errorText(error, "The case was not closed. Try again."));
    setKase(data);
    setNote("");
  }

  if (!kase) {
    return error ? <Notice tone="error">{error}</Notice> : <p className="text-stone-500">Loading the case…</p>;
  }

  const open = kase.status === "OPEN";
  const isOpener = staff?.signedIn === true && staff.name === kase.opened_by;
  const canClose = open && hasRole(staff ?? null, "compliance");

  return (
    <section aria-labelledby="case-heading" className="space-y-6">
      <p>
        <Link href="/cases" className="text-sm text-sky-700 underline">
          Back to cases
        </Link>
      </p>
      <h1 id="case-heading" className="text-2xl font-semibold">
        Case {kase.case_id.slice(0, 8)}
      </h1>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {kase.overdue ? <Notice tone="error">This case is past its SLA.</Notice> : null}

      <dl className="grid max-w-2xl grid-cols-2 gap-x-6 gap-y-2 rounded-lg bg-white p-5 text-sm shadow-sm">
        <dt className="text-stone-500">Rule</dt>
        <dd>{kase.rule}</dd>
        <dt className="text-stone-500">Severity</dt>
        <dd>{kase.severity}</dd>
        <dt className="text-stone-500">Status</dt>
        <dd data-testid="case-status">{open ? "Open" : "Closed"}</dd>
        <dt className="text-stone-500">Opened by</dt>
        <dd>{kase.opened_by}</dd>
        <dt className="text-stone-500">Due</dt>
        <dd>{new Date(kase.sla_due_at ?? "").toLocaleString("en-ZA")}</dd>
        <dt className="text-stone-500">Detail</dt>
        <dd className="col-span-1">{kase.detail}</dd>
        <dt className="text-stone-500">Evidence</dt>
        <dd>{(kase.evidence ?? []).length ? (kase.evidence ?? []).join(", ") : "None recorded"}</dd>
        {!open ? (
          <>
            <dt className="text-stone-500">Closed by</dt>
            <dd>{kase.closed_by}</dd>
            <dt className="text-stone-500">Closing note</dt>
            <dd className="col-span-1">{kase.closing_note}</dd>
          </>
        ) : null}
      </dl>

      {open && !hasRole(staff ?? null, "compliance") ? (
        <Notice tone="info">Only a compliance role can close a case.</Notice>
      ) : null}
      {canClose && isOpener ? (
        <Notice tone="info">You opened this case. Another compliance person must close it.</Notice>
      ) : null}

      {canClose && !isOpener ? (
        <form onSubmit={close} className="max-w-2xl space-y-3" aria-label="Close this case">
          <label className="block text-sm">
            <span className="font-medium">Closing note (at least {MIN_NOTE} characters)</span>
            <textarea
              name="note"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              rows={4}
              required
              className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2 text-sm"
            />
          </label>
          <button
            type="submit"
            disabled={busy || note.trim().length < MIN_NOTE}
            className="rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white hover:bg-stone-800 disabled:opacity-50"
          >
            {busy ? "Closing…" : "Close case"}
          </button>
        </form>
      ) : null}
    </section>
  );
}
