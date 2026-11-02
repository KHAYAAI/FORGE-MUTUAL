import { expect, test } from "@playwright/test";
import { mockAccounts, reply, signIn } from "./helpers";

const quote = (expiresAt: string) => ({
  quote_id: "q-1",
  sell_asset: "ZAR",
  buy_asset: "USD",
  sell_amount_minor: "1000000",
  buy_amount_minor: "5400",
  rate_scaled: "1850000000",
  mid_rate_scaled: "1825000000",
  spread_bps: 50,
  spread_minor: "9125",
  expires_at: expiresAt,
  disclosure: "You pay R 10 000.00. You receive US$ 54.00 at 18.50 ZAR per USD, which includes our spread of R 91.25.",
});

const future = () => new Date(Date.now() + 60_000).toISOString();

test("a quote shows what is paid, what is received and the disclosure, and cannot be booked until accepted", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);
  await page.route("**/api/v1/fx/quotes", (route) => reply(route, 200, quote(future())));

  await page.goto("/fx");
  await page.getByLabel("Amount (ZAR)").fill("10000");
  await page.getByRole("button", { name: "Get a quote" }).click();

  const panel = page.getByLabel("Your quote");
  await expect(panel.locator("dd").filter({ hasText: "R 10 000.00" })).toBeVisible();
  await expect(panel.locator("dd").filter({ hasText: "US$ 54.00" })).toBeVisible();
  await expect(page.getByTestId("disclosure")).toContainText("includes our spread of R 91.25");

  const book = page.getByRole("button", { name: "Book this conversion" });
  await expect(book).toBeDisabled();
  await page.getByRole("checkbox").check();
  await expect(book).toBeEnabled();
});

test("booking sends the quote with the chosen accounts, and shows the deal", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);
  await page.route("**/api/v1/fx/quotes", (route) => reply(route, 200, quote(future())));
  let booked: Record<string, unknown> | null = null;
  await page.route("**/api/v1/fx/deals", (route) => {
    booked = JSON.parse(route.request().postData() ?? "{}");
    return reply(route, 202, { deal_id: "deal-7", state: "BOOKED" });
  });

  await page.goto("/fx");
  await page.getByLabel("Amount (ZAR)").fill("10000");
  await page.getByRole("button", { name: "Get a quote" }).click();
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "Book this conversion" }).click();

  await expect(page.getByTestId("notice")).toContainText("deal-7");
  expect(booked).toEqual({ quote_id: "q-1", sell_account_id: "acc-zar", buy_account_id: "acc-usd" });
});

test("an expired quote cannot be booked", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);
  await page.route("**/api/v1/fx/quotes", (route) =>
    reply(route, 200, quote(new Date(Date.now() - 1_000).toISOString())),
  );

  await page.goto("/fx");
  await page.getByLabel("Amount (ZAR)").fill("10000");
  await page.getByRole("button", { name: "Get a quote" }).click();
  await expect(page.getByTestId("notice").filter({ hasText: "expired" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Book this conversion" })).toHaveCount(0);
});
