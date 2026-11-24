import { expect, test } from "@playwright/test";
import { reply, signIn, zarAccount } from "./helpers";

// A product that is switched off is not advertised: its link appears only while its route answers 200.
test("Cards and Invest links are absent while those products are off", async ({ page }) => {
  await page.route("**/api/v1/accounts", (route) => reply(route, 200, { accounts: [zarAccount] }));
  await page.route("**/api/v1/cards", (route) => reply(route, 404, { error: { code: "not_found", message: "cards are not offered here" } }));
  await page.route("**/api/v1/offers", (route) => reply(route, 404, { error: { code: "not_found", message: "third-party offers are not offered here" } }));
  await signIn(page);
  await expect(page.getByRole("link", { name: "Credit", exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Cards", exact: true })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Invest", exact: true })).toHaveCount(0);
});

test("each link appears only for the product that is on", async ({ page }) => {
  await page.route("**/api/v1/accounts", (route) => reply(route, 200, { accounts: [zarAccount] }));
  await page.route("**/api/v1/cards", (route) => reply(route, 200, { cards: [] }));
  await page.route("**/api/v1/offers", (route) => reply(route, 404, { error: { code: "not_found", message: "third-party offers are not offered here" } }));
  await signIn(page);
  await expect(page.getByRole("link", { name: "Cards", exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Invest", exact: true })).toHaveCount(0);

  await page.route("**/api/v1/offers", (route) => reply(route, 200, { offers: [] }));
  await page.reload();
  await expect(page.getByRole("link", { name: "Invest", exact: true })).toBeVisible();
});
