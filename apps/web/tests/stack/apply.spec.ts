import { randomBytes } from "node:crypto";
import { expect, test } from "@playwright/test";
import { findApplication, nationalId, verdict, waitForAccounts, waitForState } from "../../../../deploy/compose/journeys/seed.cjs";

// A personal application through the front door, against the real local stack. The browser signs in, applies and
// watches its status; workspace-api, onboarding (OnboardPerson on Temporal), party, consent, policy, screening and the
// ledger do the rest. The one outside party is the identity provider: until Didit is connected the stack uses the
// provider adapter's local mode, and this test delivers its verdict as a signed webhook, exactly as Didit would.

test("a person applies, verifies, and the account they open is theirs", async ({ page }) => {
  const name = `Applicant ${randomBytes(4).toString("hex")}`;
  const subject = "dev-" + name.toLowerCase().replace(/[^a-z0-9]+/g, "-");

  await page.goto("/apply");
  await page.getByRole("link", { name: "Apply" }).first().click();
  await page.getByRole("link", { name: "Sign in to apply" }).click();
  await page.getByLabel("Your name").fill(name);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/apply/personal");

  await page.getByLabel("Full legal name").fill(name);
  await page.getByLabel("South African ID number").fill(nationalId());
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "Continue" }).click();
  await page.waitForURL("**/apply/status/person");

  // The real workflow creates the person and starts their verification; the page offers the verification link.
  const application = await findApplication(subject, "person");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("One more step: verify your identity", { timeout: 30_000 });
  await expect(page.getByRole("link", { name: "Start verification" })).toBeVisible();

  // Sending the form again does not open a second application.
  const again = await page.request.post("/api/v1/applications/person", {
    data: { legal_name: name, national_id: nationalId() },
  });
  expect(again.status()).toBeLessThan(500);
  expect((await findApplication(subject, "person")).onboardingId).toBe(application.onboardingId);

  // The provider approves; screening passes; the account is opened in the ledger.
  await verdict(application.onboardingId, application.personPartyIds![0], "APPROVED");
  const done = await waitForState(application.onboardingId, "ONBOARDING_STATE_COMPLETED", 90_000);
  expect(done.accountIds?.length).toBe(1);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Your account is open", { timeout: 30_000 });

  // The account reaches its owner once reconciliation has named them (fail closed until then).
  await waitForAccounts(application.personPartyIds![0], 120_000);
  await page.getByRole("link", { name: "Go to your account" }).click();
  await page.waitForURL("**/accounts", { timeout: 30_000 });
  await expect(page.getByRole("listitem").filter({ hasText: "ZAR account" })).toHaveCount(1);
});

test("a declined applicant is told so, and never why", async ({ page }) => {
  const name = `Declined ${randomBytes(4).toString("hex")}`;
  const subject = "dev-" + name.toLowerCase().replace(/[^a-z0-9]+/g, "-");
  await page.goto("/sign-in?next=/apply/personal");
  await page.getByLabel("Your name").fill(name);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/apply/personal");
  await page.getByLabel("Full legal name").fill(name);
  await page.getByLabel("South African ID number").fill(nationalId());
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "Continue" }).click();
  await page.waitForURL("**/apply/status/person");

  const application = await findApplication(subject, "person");
  await verdict(application.onboardingId, application.personPartyIds![0], "DECLINED");
  await waitForState(application.onboardingId, "ONBOARDING_STATE_DECLINED", 60_000);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("We could not open your account", { timeout: 30_000 });
  const api = await (await page.request.get("/api/v1/applications/person")).json();
  expect(JSON.stringify(api)).not.toMatch(/reason|kyc|sanction/i);
});
