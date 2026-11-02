import { expect, test } from "@playwright/test";
import { startCompany, verdict, waitForReview, waitForState } from "../../../../deploy/compose/journeys/seed.cjs";
import { signIn } from "../helpers";

// The back-office journey against the real local stack (deploy/compose). The provider's REVIEW verdict is delivered
// as the provider would deliver it; the review appears in the compliance queue of the real onboarding service, a
// compliance person decides it in the app, and the real onboarding workflow resumes and completes.

test("a provider's review is decided by a compliance person, and the onboarding completes", async ({ page }) => {
  const company = await startCompany("staff");
  await verdict(company.onboardingId, company.companyPartyId, "APPROVED");
  await verdict(company.onboardingId, company.personPartyIds[0], "REVIEW");
  const review = await waitForReview(company.onboardingId, company.personPartyIds[0]);

  await signIn(page, "Naledi", ["compliance"]);
  await page.goto("/kyc");
  const card = page.getByRole("article", { name: `Review ${review.review_id.slice(0, 8)}` });
  await expect(card).toBeVisible();
  await expect(card.getByText("provider could not decide", { exact: true })).toBeVisible();

  await card.getByLabel(/What did you check/).fill("Checked the register extract and the ID document against the applicant.");
  await card.getByRole("button", { name: "Record decision" }).click();
  await expect(page.getByTestId("notice")).toHaveText("Review approved. The onboarding continues.");

  await waitForState(company.onboardingId, "ONBOARDING_STATE_COMPLETED");
});
