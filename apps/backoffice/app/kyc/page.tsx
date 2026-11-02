"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { hasRole, useStaff } from "@/lib/useStaff";

type Review = components["schemas"]["KycReview"];
const MIN_NOTE = 20;

export default function KycPage() {
  const router = useRouter();
  const staff = useStaff("/kyc");
  const [reviews, setReviews] = useState<Review[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  async function load() {
    const { data, error, response } = await api.GET("/v1/kyc/reviews", { params: { query: { open_only: true } } });
    if (response.status === 401) return router.push("/sign-in");
    if (error || !data) return setError(errorText(error, "The reviews could not be loaded. Try again."));
    setError(null);
    setReviews(data.reviews);
  }

  useEffect(() => {
    void load();
    // load reads the list once per visit; a decision reloads it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router]);

  const canDecide = hasRole(staff ?? null, "compliance");

  return (
    <section aria-labelledby="kyc-heading" className="space-y-6">
      <h1 id="kyc-heading" className="text-2xl font-semibold">
        KYC reviews
      </h1>
      <p className="text-sm text-stone-600">
        A provider could not decide these checks. A person decides each one; the onboarding waits for the decision.
      </p>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {notice ? <Notice tone="success">{notice}</Notice> : null}
      {!canDecide && staff?.signedIn ? <Notice tone="info">A compliance role is needed to decide KYC reviews.</Notice> : null}
      {reviews?.length === 0 ? <p className="text-stone-600">No reviews are waiting.</p> : null}
      {reviews?.map((rv) => (
        <ReviewCard
          key={rv.review_id}
          review={rv}
          canDecide={canDecide}
          onDecided={(msg) => {
            setNotice(msg);
            void load();
          }}
        />
      ))}
    </section>
  );
}

function ReviewCard({
  review,
  canDecide,
  onDecided,
}: {
  review: Review;
  canDecide: boolean;
  onDecided: (message: string) => void;
}) {
  const [decision, setDecision] = useState<"APPROVED" | "DECLINED">("APPROVED");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    const { error } = await api.POST("/v1/kyc/reviews/{review}/decision", {
      params: { path: { review: review.review_id } },
      body: { decision, note: note.trim() },
    });
    setBusy(false);
    if (error) return setError(errorText(error, "The decision was not recorded. Try again."));
    onDecided(decision === "APPROVED" ? "Review approved. The onboarding continues." : "Review declined. The onboarding ends.");
  }

  return (
    <article className="space-y-4 rounded-lg bg-white p-5 text-sm shadow-sm" aria-label={`Review ${review.review_id.slice(0, 8)}`}>
      <dl className="grid grid-cols-2 gap-x-6 gap-y-2">
        <dt className="text-stone-500">Review</dt>
        <dd className="font-mono">{review.review_id.slice(0, 8)}</dd>
        <dt className="text-stone-500">Party</dt>
        <dd className="font-mono">{review.subject_party_id}</dd>
        <dt className="text-stone-500">Reason</dt>
        <dd>{review.reason}</dd>
        <dt className="text-stone-500">Opened</dt>
        <dd>{new Date(review.opened_at ?? "").toLocaleString("en-ZA")}</dd>
      </dl>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {canDecide ? (
        <form onSubmit={submit} className="space-y-3" aria-label="Decide this review">
          <label className="block">
            <span className="font-medium">Decision</span>
            <select
              name="decision"
              value={decision}
              onChange={(e) => setDecision(e.target.value as "APPROVED" | "DECLINED")}
              className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2"
            >
              <option value="APPROVED">Approve</option>
              <option value="DECLINED">Decline</option>
            </select>
          </label>
          <label className="block">
            <span className="font-medium">What did you check, and why? (at least {MIN_NOTE} characters)</span>
            <textarea
              name="note"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              rows={3}
              required
              className="mt-1 w-full rounded-md border border-stone-300 px-3 py-2"
            />
          </label>
          <button
            type="submit"
            disabled={busy || note.trim().length < MIN_NOTE}
            className="rounded-md bg-stone-900 px-4 py-2 font-medium text-white hover:bg-stone-800 disabled:opacity-50"
          >
            {busy ? "Recording…" : "Record decision"}
          </button>
        </form>
      ) : null}
    </article>
  );
}
