import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { reply, signIn, zarAccount } from "./helpers";

const TOKEN = "abcdefghijklmnopqrstuvwxyz234567";
const link = {
  link_id: "lnk-1", token: TOKEN, channel: "link", amount_minor: "250000", description: "Design sprint", reference: "INV-77",
  status: "OPEN", created_at: "2026-11-17T09:00:00Z", expires_at: "2026-11-24T09:00:00Z", paid_at: "", refunded_minor: "0",
  settlement_account_id: "acc-zar", merchant_name: "Forge Pilot Studio",
};

test("a merchant creates a payment link, and the payer pays it on the acquirer's page", async ({ page }) => {
  let created: unknown = null;
  let links: unknown[] = [];
  let status = "OPEN";
  await page.route("**/api/v1/accounts", (route) => reply(route, 200, { accounts: [zarAccount] }));
  await page.route("**/api/v1/settlements", (route) => reply(route, 200, { settlements: [], mdr_bps: 275 }));
  await page.route("**/api/v1/payment-links", (route) => {
    if (route.request().method() === "POST") {
      created = route.request().postDataJSON();
      links = [link];
      return reply(route, 201, link);
    }
    return reply(route, 200, { links });
  });
  await signIn(page);
  await page.getByRole("link", { name: "Get paid" }).click();
  await expect(page.getByText("less a 2.75% card fee")).toBeVisible();
  const form = page.getByRole("form", { name: "New payment request" });
  await form.getByLabel(/business name/).fill("Forge Pilot Studio");
  await form.getByLabel("Amount (R)").fill("2500");
  await form.getByLabel("What it is for").fill("Design sprint");
  await form.getByLabel(/Your reference/).fill("INV-77");
  await form.getByRole("button", { name: "Create" }).click();
  await expect(page.getByText("Your payment link is ready.")).toBeVisible();
  expect(created).toEqual({ settlement_account_id: "acc-zar", amount_minor: "250000", description: "Design sprint", reference: "INV-77", channel: "link", merchant_name: "Forge Pilot Studio" });
  const a11y = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(a11y.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => v.id)).toEqual([]);

  // The payer, signed out, in a new context: the pay page needs no sign-in.
  const payerContext = await page.context().browser()!.newContext({ baseURL: "http://127.0.0.1:3100" });
  const payer = await payerContext.newPage();
  await payer.route(`**/api/checkout/${TOKEN}`, (route) => reply(route, 200, { merchant_name: "Forge Pilot Studio", amount_minor: "250000", description: "Design sprint", reference: "INV-77", status, expires_at: link.expires_at }));
  await payer.route(`**/api/checkout/${TOKEN}/start`, (route) => reply(route, 200, { checkout_url: `/pay/${TOKEN}/simulator`, simulated: true }));
  await payer.route(`**/api/checkout/${TOKEN}/simulate`, (route) => {
    status = "PAID";
    return reply(route, 200, { status: "APPROVED" });
  });
  await payer.goto(`/pay/${TOKEN}`);
  await expect(payer.getByRole("heading", { name: "R 2 500.00" })).toBeVisible();
  await expect(payer.getByText("Pay Forge Pilot Studio")).toBeVisible();
  await expect(payer.getByText("Forge never sees your card details.")).toBeVisible();
  const pa11y = await new AxeBuilder({ page: payer }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(pa11y.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => v.id)).toEqual([]);
  await payer.getByRole("button", { name: "Pay by card" }).click();
  await payer.waitForURL(`**/pay/${TOKEN}/simulator`);
  await expect(payer.getByText("no card is asked for and nothing is charged")).toBeVisible();
  await payer.getByRole("button", { name: "Approve test payment" }).click();
  await payer.waitForURL(`**/pay/${TOKEN}`);
  await expect(payer.getByText("Paid. Thank you.")).toBeVisible();
});

test("a malformed token never reaches the service", async ({ request }) => {
  const res = await request.get("/api/checkout/..%2F..%2Fv1%2Faccounts");
  expect(res.status()).toBe(404);
});
