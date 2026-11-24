import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { reply, signIn } from "./helpers";

const provider = {
  provider_id: "acme", name: "Acme Wealth (Pty) Ltd", licence_type: "FSP", regulator: "FSCA", licence_number: "FSP 12345", status: "PENDING",
  licence_verified_at: "", licence_verified_by: "", licence_expires_at: "",
};
const statement = "THIRD-PARTY PRODUCT: provided, licensed and monitored by Acme Wealth (Pty) Ltd, not by Forge Mutual.";
const adminOffer = (status: string, visible = false) => ({
  offer: { offer_id: "acme-tfsa", provider_id: "acme", provider_name: "Acme Wealth (Pty) Ltd", category: "INVESTMENTS", title: "Acme Tax-Free Savings", summary: "s", statement, fee_disclosure: "None" },
  status, handoff_url: "https://invest.acme-wealth.example/start", updated_by: "Anna", visible_now: visible,
});

test("third-party providers are not shown when offers are not switched on, and the nav says nothing about them", async ({ page }) => {
  await page.route("**/api/v1/partners/**", (route) => reply(route, 404, { error: { code: "not_found", message: "third-party offers are not offered here" } }));
  await signIn(page, "Anna", ["compliance"]);
  await expect(page.getByRole("link", { name: "Partners" })).toHaveCount(0);
  await page.goto("/partners");
  await expect(page.getByText("Third-party offers are not switched on.")).toBeVisible();
});

test("compliance registers a provider, a second person verifies it, and an offer is switched on after its statement is read", async ({ page }) => {
  let providers: unknown[] = [];
  let offers: unknown[] = [];
  const sent: Record<string, unknown> = {};
  await page.route("**/api/v1/partners/providers", (route) => {
    if (route.request().method() === "POST") {
      sent.register = route.request().postDataJSON();
      providers = [provider];
      return reply(route, 201, provider);
    }
    return reply(route, 200, { providers });
  });
  await page.route("**/api/v1/partners/providers/acme/verify", (route) => {
    sent.verify = route.request().postDataJSON();
    providers = [{ ...provider, status: "ACTIVE", licence_verified_at: "2027-03-01T10:00:00Z", licence_verified_by: "Ben" }];
    return reply(route, 200, providers[0]);
  });
  await page.route("**/api/v1/partners/offers", (route) => {
    if (route.request().method() === "POST") {
      sent.offer = route.request().postDataJSON();
      offers = [adminOffer("DRAFT")];
      return reply(route, 200, adminOffer("DRAFT").offer);
    }
    return reply(route, 200, { offers });
  });
  await page.route("**/api/v1/partners/offers/acme-tfsa/status", (route) => {
    sent.status = route.request().postDataJSON();
    offers = [adminOffer("ACTIVE", true)];
    return reply(route, 200, { status: "ACTIVE" });
  });

  await signIn(page, "Anna", ["compliance"]);
  await expect(page.getByRole("link", { name: "Partners" })).toBeVisible();
  await page.getByRole("link", { name: "Partners" }).click();
  await expect(page.getByRole("heading", { name: "Third-party providers" })).toBeVisible();

  const reg = page.getByRole("form", { name: "Register a provider" });
  await reg.getByLabel("Short name (lower case, for the system)").fill("acme");
  await reg.getByLabel("Legal name").fill("Acme Wealth (Pty) Ltd");
  await reg.getByLabel("Licence number").fill("FSP 12345");
  await reg.getByLabel(/Link to the regulator/).fill("https://www.fsca.co.za/register/acme");
  await reg.getByRole("button", { name: "Register provider" }).click();
  await expect(page.getByText("Someone else must now verify its licence.", { exact: false })).toBeVisible();
  expect(sent.register).toMatchObject({ provider_id: "acme", licence_type: "FSP", regulator: "FSCA", licence_number: "FSP 12345" });
  const row = page.getByRole("listitem", { name: "Acme Wealth (Pty) Ltd" });
  await expect(row.getByTestId("provider-status")).toHaveText("PENDING");

  await row.getByRole("button", { name: /verify licence/ }).click();
  await expect(page.getByText("Acme Wealth (Pty) Ltd: licence verified.")).toBeVisible();
  await expect(row.getByTestId("provider-status")).toHaveText("ACTIVE");
  expect(sent.verify).toEqual({ licence_expires_at: "" });

  const form = page.getByRole("form", { name: "Write an offer" });
  await form.getByLabel(/Short name/).fill("acme-tfsa");
  await form.getByLabel("Title").fill("Acme Tax-Free Savings");
  await form.getByLabel("Summary").fill("s");
  await form.getByLabel(/provider's own address/).fill("https://invest.acme-wealth.example/start");
  await form.getByLabel(/What, if anything/).fill("None");
  await form.getByRole("button", { name: "Save as draft" }).click();
  await expect(page.getByText("Someone else must read its statement and switch it on.", { exact: false })).toBeVisible();
  expect(sent.offer).toMatchObject({ offer_id: "acme-tfsa", provider_id: "acme", category: "INVESTMENTS" });

  const offer = page.getByRole("listitem", { name: "Acme Tax-Free Savings" });
  await expect(offer.getByTestId("offer-status")).toHaveText("DRAFT");
  await offer.getByText("The statement customers will be shown").click();
  await expect(offer).toContainText("provided, licensed and monitored by Acme Wealth (Pty) Ltd, not by Forge Mutual.");
  const a11y = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(a11y.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => v.id)).toEqual([]);

  await offer.getByRole("button", { name: "I read the statement: switch on" }).click();
  await expect(page.getByText("Acme Tax-Free Savings is switched on.")).toBeVisible();
  expect(sent.status).toEqual({ status: "ACTIVE" });
  await expect(offer.getByText("customers can see it now")).toBeVisible();
});

test("staff without the compliance role see no Partners link and are told they cannot manage providers", async ({ page }) => {
  await page.route("**/api/v1/partners/**", (route) => reply(route, 403, { error: { code: "forbidden", message: "forbidden" } }));
  await signIn(page, "Sipho", ["credit"]);
  await expect(page.getByRole("link", { name: "Partners" })).toHaveCount(0);
  await page.goto("/partners");
  await expect(page.getByText("You need the compliance role to manage third-party providers.")).toBeVisible();
});
