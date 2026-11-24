import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { reply, signIn, zarAccount } from "./helpers";

const card = {
  card_id: "card-1", party_id: "p", account_id: "acc-zar", label: "Subscriptions", last4: "4242", expiry_month: 3, expiry_year: 2030,
  status: "ACTIVE", per_transaction_limit_minor: "1000000", daily_limit_minor: "2500000", created_at: "2027-03-01T10:00:00Z", network: "SIMULATED",
};

test("cards are not yet available when the bank has not switched them on", async ({ page }) => {
  await page.route("**/api/v1/accounts", (route) => reply(route, 200, { accounts: [zarAccount] }));
  await page.route("**/api/v1/cards", (route) => reply(route, 404, { error: { code: "not_found", message: "cards are not offered here" } }));
  await signIn(page);
  await page.goto("/cards");
  await expect(page.getByText("Cards are not yet available.")).toBeVisible();
  await expect(page.getByRole("form", { name: "Get a card" })).toHaveCount(0);
});

test("a customer gets a card, freezes it, changes its limits and sees its purchases", async ({ page }) => {
  let cards: unknown[] = [];
  let issued: unknown = null;
  let limits: unknown = null;
  await page.route("**/api/v1/accounts", (route) => reply(route, 200, { accounts: [zarAccount] }));
  await page.route("**/api/v1/cards", (route) => {
    if (route.request().method() === "POST") {
      issued = route.request().postDataJSON();
      cards = [card];
      return reply(route, 201, card);
    }
    return reply(route, 200, { cards });
  });
  await page.route("**/api/v1/cards/card-1/status", (route) => {
    const status = (route.request().postDataJSON() as { status: string }).status;
    cards = [{ ...card, status }];
    return reply(route, 200, cards[0]);
  });
  await page.route("**/api/v1/cards/card-1/limits", (route) => {
    limits = route.request().postDataJSON();
    cards = [{ ...card, per_transaction_limit_minor: "500000", daily_limit_minor: "1000000" }];
    return reply(route, 200, cards[0]);
  });
  await page.route("**/api/v1/cards/card-1/authorisations", (route) =>
    reply(route, 200, {
      authorisations: [
        { auth_id: "a1", card_id: "card-1", merchant_name: "Acme Cloud", amount_minor: "30000", status: "CAPTURED", currency: "ZAR" },
        { auth_id: "a2", card_id: "card-1", merchant_name: "Casino", amount_minor: "5000", status: "DECLINED", decline_reason: "MERCHANT_CATEGORY_BLOCKED", currency: "ZAR" },
      ],
    }),
  );

  await signIn(page);
  await page.goto("/cards");
  const form = page.getByRole("form", { name: "Get a card" });
  await form.getByLabel("Name").fill("Subscriptions");
  await form.getByRole("button", { name: "Get card" }).click();
  await expect(page.getByText("Your card is ready.")).toBeVisible();
  expect(issued).toEqual({ account_id: "acc-zar", label: "Subscriptions" });

  const item = page.getByRole("listitem", { name: "Subscriptions" });
  await expect(item.getByText("•••• 4242")).toBeVisible();
  await expect(item.getByText("test card", { exact: false })).toBeVisible();
  const a11y = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(a11y.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => v.id)).toEqual([]);

  await item.getByRole("button", { name: "Freeze" }).click();
  await expect(page.getByText("The card is frozen.")).toBeVisible();
  await expect(item.getByRole("button", { name: "Unfreeze" })).toBeVisible();
  await item.getByRole("button", { name: "Unfreeze" }).click();
  await expect(page.getByText("The card is active again.")).toBeVisible();

  await item.getByLabel("Per purchase (R)").fill("5000");
  await item.getByLabel("Per day (R)").fill("10000");
  await item.getByRole("button", { name: "Save limits" }).click();
  await expect(page.getByText("Limits saved.")).toBeVisible();
  expect(limits).toEqual({ per_transaction_limit_minor: "500000", daily_limit_minor: "1000000" });

  await item.getByRole("button", { name: "Show purchases" }).click();
  await expect(item.getByText("Acme Cloud · Paid")).toBeVisible();
  await expect(item.getByText("This type of merchant is not allowed.", { exact: false })).toBeVisible();
});
