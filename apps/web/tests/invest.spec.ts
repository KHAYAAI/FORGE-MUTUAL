import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { reply, signIn } from "./helpers";

const statement =
  "THIRD-PARTY PRODUCT: provided, licensed and monitored by Acme Wealth (Pty) Ltd, not by Forge Mutual.\n\nAcme Tax-Free Savings is a product of Acme Wealth (Pty) Ltd, which is licensed by the FSCA as a financial services provider (licence number FSP 12345).";
const offer = {
  offer_id: "acme-tfsa", category: "INVESTMENTS", title: "Acme Tax-Free Savings", summary: "A tax-free savings account.", provider_name: "Acme Wealth (Pty) Ltd",
  licence_type: "FSP", licence_number: "FSP 12345", regulator: "FSCA", provider_id: "acme", statement, statement_hash: "hash-1", fee_disclosure: "Forge receives no fee.",
};

test("third-party products are not yet available when the bank has not switched them on", async ({ page }) => {
  await page.route("**/api/v1/offers", (route) => reply(route, 404, { error: { code: "not_found", message: "third-party offers are not offered here" } }));
  await signIn(page);
  await page.goto("/invest");
  await expect(page.getByText("Third-party products are not yet available.")).toBeVisible();
});

test("the third-party statement is shown in full and must be acknowledged before the customer is introduced", async ({ page }) => {
  let referral: unknown = null;
  let consent: unknown = null;
  let consentGranted = false;
  const view = () => ({ granted: consentGranted, consent_id: consentGranted ? "consent-1" : "", provider_name: "Acme Wealth (Pty) Ltd", shares: ["your name"] });
  await page.route("**/api/v1/offers", (route) => reply(route, 200, { offers: [offer] }));
  await page.route("**/api/v1/offers/acme-tfsa/consent", (route) => {
    if (route.request().method() === "POST") {
      consent = route.request().postDataJSON();
      consentGranted = true;
      return reply(route, 201, view());
    }
    return reply(route, 200, view());
  });
  await page.route("**/api/v1/offers/acme-tfsa/consent/withdraw", (route) => {
    consentGranted = false;
    return reply(route, 200, view());
  });
  await page.route("**/api/v1/offers/acme-tfsa/referrals", (route) => {
    referral = route.request().postDataJSON();
    return reply(route, 201, { referral_id: "ref-1", offer_id: "acme-tfsa", provider_name: "Acme Wealth (Pty) Ltd", handoff_url: "https://invest.acme-wealth.example/start?ref=ref-1" });
  });
  await signIn(page);
  await page.goto("/invest");
  await expect(page.getByText("provided, licensed and monitored by other companies, not by Forge Mutual", { exact: false })).toBeVisible();
  const card = page.getByRole("listitem", { name: "Acme Tax-Free Savings" });
  const box = card.getByRole("region", { name: "Important: about Acme Tax-Free Savings" });
  await expect(box).toContainText("THIRD-PARTY PRODUCT: provided, licensed and monitored by Acme Wealth (Pty) Ltd, not by Forge Mutual.");
  await expect(box).toContainText("licence number FSP 12345");
  const a11y = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(a11y.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => v.id)).toEqual([]);

  const go = card.getByRole("button", { name: "Continue to Acme Wealth (Pty) Ltd" });
  const sharing = card.getByRole("group", { name: "Sharing your details with Acme Wealth (Pty) Ltd" });
  await expect(sharing).toContainText("your name");
  await expect(sharing).not.toContainText("email");
  await expect(sharing).toContainText("cannot take back details already sent");
  await expect(go).toBeDisabled();
  await expect(sharing.getByRole("button", { name: "I agree to share these details" })).toBeDisabled();
  await card.getByRole("checkbox").check();
  await expect(go).toBeDisabled(); // read, but no consent to share yet
  await sharing.getByRole("button", { name: "I agree to share these details" }).click();
  await expect(sharing.getByText("You agreed to share these details with Acme Wealth (Pty) Ltd.")).toBeVisible();
  expect(consent).toEqual({ agrees: true, statement_hash: "hash-1" });
  await sharing.getByRole("button", { name: "Withdraw my consent" }).click();
  await expect(go).toBeDisabled();
  await sharing.getByRole("button", { name: "I agree to share these details" }).click();
  await go.click();
  expect(referral).toEqual({ acknowledged_third_party: true, statement_hash: "hash-1" });
  await expect(page.getByRole("link", { name: "go to Acme Wealth (Pty) Ltd" })).toHaveAttribute("href", "https://invest.acme-wealth.example/start?ref=ref-1");
});
