import { expect, test } from "@playwright/test";
import { reply, signIn } from "./helpers";

const REVIEW = "44444444-4444-4444-8444-444444444444";
const review = {
  review_id: REVIEW,
  onboarding_id: "onb-1",
  subject_party_id: "55555555-5555-4555-8555-555555555555",
  reason: "provider could not decide",
  status: "OPEN",
  opened_at: "2026-11-01T08:00:00Z",
  decision: "",
  reviewer: "",
  note: "",
};

test("a compliance person sees open reviews and records a decision with a note", async ({ page }) => {
  let reviews = [review];
  await page.route("**/api/v1/kyc/reviews?*", (route) => reply(route, 200, { reviews }));
  let decided: { decision?: string; note?: string } | null = null;
  await page.route(`**/api/v1/kyc/reviews/${REVIEW}/decision`, (route) => {
    decided = JSON.parse(route.request().postData() ?? "{}");
    reviews = [];
    return reply(route, 200, { ...review, status: "DECIDED", decision: "APPROVED", reviewer: "Naledi" });
  });

  await signIn(page, "Naledi", ["compliance"]);
  await page.goto("/kyc");
  await expect(page.getByText("provider could not decide", { exact: true })).toBeVisible();

  const card = page.getByRole("article", { name: "Review 44444444" });
  await card.getByLabel(/What did you check/).fill("Checked the register extract and the ID document against the applicant.");
  await card.getByRole("button", { name: "Record decision" }).click();

  await expect(page.getByTestId("notice")).toHaveText("Review approved. The onboarding continues.");
  expect(decided).toEqual({ decision: "APPROVED", note: "Checked the register extract and the ID document against the applicant." });
});

test("an operations person sees the reviews but cannot decide them", async ({ page }) => {
  await page.route("**/api/v1/kyc/reviews?*", (route) => reply(route, 200, { reviews: [review] }));
  await signIn(page, "Sipho", ["operations"]);
  await page.goto("/kyc");
  await expect(page.getByText("A compliance role is needed to decide KYC reviews.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Record decision" })).toHaveCount(0);
});

test("a decision the server refuses is shown, and the review stays as it was", async ({ page }) => {
  await page.route("**/api/v1/kyc/reviews?*", (route) => reply(route, 200, { reviews: [review] }));
  await page.route(`**/api/v1/kyc/reviews/${REVIEW}/decision`, (route) =>
    reply(route, 409, { error: { code: "failed_precondition", message: "the review is already decided" } }),
  );
  await signIn(page, "Naledi", ["compliance"]);
  await page.goto("/kyc");
  const card = page.getByRole("article", { name: "Review 44444444" });
  await card.getByLabel(/What did you check/).fill("Checked the register extract and the ID document against the applicant.");
  await card.getByRole("button", { name: "Record decision" }).click();
  await expect(card.getByTestId("notice")).toHaveText("the review is already decided");
});
