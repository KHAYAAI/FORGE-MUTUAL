import { expect, test, type Page } from "@playwright/test";
import { startCompany, verdict, waitForAccounts, waitForState } from "../../../../deploy/compose/journeys/seed.cjs";

// Customer journeys against the real local stack (deploy/compose). Nothing is mocked: the sign-in is the development
// one, and every balance and account comes from workspace-api, consolidated-view and the ledger.

async function signInAs(page: Page, partyId: string) {
  await page.goto("/sign-in");
  await page.getByLabel("Party ID").fill(partyId);
  await page.getByLabel("Your name").fill("Thandi");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/accounts");
}

test("an account the onboarding opened is in the company's workspace", async ({ page }) => {
  // A company is onboarded through the real onboarding workflow: the provider approves both subjects, and the
  // onboarding opens its account through the ledger.
  const company = await startCompany("customer");
  await verdict(company.onboardingId, company.companyPartyId, "APPROVED");
  await verdict(company.onboardingId, company.personPartyIds[0], "APPROVED");
  await waitForState(company.onboardingId, "ONBOARDING_STATE_COMPLETED");
  // The account is visible to its owner once reconciliation has named the owner (fail closed until then).
  await waitForAccounts(company.companyPartyId);

  await signInAs(page, company.companyPartyId);
  const zar = page.getByRole("listitem").filter({ hasText: "ZAR account" });
  await expect(zar).toHaveCount(1);
  await expect(zar.getByTestId("holding-entity")).toHaveText("BANK");
  await expect(zar.getByTestId("insured")).toHaveText("Not insured");
});

test("a party with no accounts sees none, and not another party's", async ({ page }) => {
  const company = await startCompany("isolation");
  await verdict(company.onboardingId, company.companyPartyId, "APPROVED");
  await verdict(company.onboardingId, company.personPartyIds[0], "APPROVED");
  await waitForState(company.onboardingId, "ONBOARDING_STATE_COMPLETED");
  await waitForAccounts(company.companyPartyId);

  // The director is a person party of the same onboarding, but the account belongs to the company.
  await signInAs(page, company.personPartyIds[0]);
  await expect(page.getByText("No accounts yet. Your accounts will appear here once they are opened.")).toBeVisible();
  await expect(page.getByRole("listitem").filter({ hasText: "ZAR account" })).toHaveCount(0);
});
