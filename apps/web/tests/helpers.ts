import type { Page, Route } from "@playwright/test";
import type { components } from "../lib/api/schema";

export const PARTY = "11111111-1111-4111-8111-111111111111";

type Account = components["schemas"]["Account"];

export const zarAccount: Account = {
  account_id: "acc-zar",
  asset: "ZAR",
  available_minor: "123456789",
  holding_entity: "BANK",
  insured: true,
  product_id: "current",
  as_of: "2026-11-01T08:00:00Z",
  source: "EVENT",
};

export const usdAccount: Account = {
  account_id: "acc-usd",
  asset: "USD",
  available_minor: "250000",
  holding_entity: "PARTNER BANK",
  insured: false,
  product_id: "usd",
  as_of: "2026-11-01T08:00:00Z",
  source: "LEDGER",
};

/** Answers a mocked workspace API call. */
export function reply(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

/** Signs in through the real sign-in page, which sets the httpOnly session cookie. */
export async function signIn(page: Page) {
  await page.goto("/sign-in");
  await page.getByLabel("Party ID").fill(PARTY);
  await page.getByLabel("Your name").fill("Thandi");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/accounts");
}

/** Mocks the accounts a signed-in party holds. Other calls on the page can be routed separately. */
export async function mockAccounts(page: Page, accounts: Account[] = [zarAccount, usdAccount]) {
  await page.route("**/api/v1/accounts", (route) => reply(route, 200, { accounts }));
}
