import { expect, test } from "@playwright/test";
import { mockAccounts, reply, signIn, usdAccount, zarAccount } from "./helpers";

test("every balance names the entity that holds it and whether it is insured", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);

  const cards = page.getByRole("listitem");
  await expect(cards).toHaveCount(2);

  const zar = cards.filter({ hasText: "ZAR account" });
  await expect(zar.getByTestId("holding-entity")).toHaveText("BANK");
  await expect(zar.getByTestId("insured")).toHaveText("Insured");

  const usd = cards.filter({ hasText: "USD account" });
  await expect(usd.getByTestId("holding-entity")).toHaveText("PARTNER BANK");
  await expect(usd.getByTestId("insured")).toHaveText("Not insured");
});

test("money is shown in full, from integer minor units, without rounding", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);

  const balances = page.getByTestId("balance");
  await expect(balances.nth(0)).toHaveText("R 1 234 567.89");
  await expect(balances.nth(1)).toHaveText("US$ 2 500.00");
});

test("an uninsured balance is labelled as not insured, not left unlabelled", async ({ page }) => {
  await mockAccounts(page, [{ ...zarAccount, insured: false, holding_entity: "BANK", source: "LEDGER" }, usdAccount]);
  await signIn(page);
  await expect(page.getByTestId("insured").first()).toHaveText("Not insured");
});

test("an expired session is sent to sign in", async ({ page }) => {
  await page.route("**/api/v1/**", (route) => reply(route, 401, { error: { code: "unauthenticated", message: "sign in to continue" } }));
  await page.goto("/accounts");
  await page.waitForURL("**/sign-in");
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
});

test("the activity of an account pages forward with the cursor", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);

  const first = Array.from({ length: 50 }, (_, i) => ({
    seq: 100 - i,
    transfer_id: `t${100 - i}`,
    direction: "IN",
    amount_minor: "100",
    asset: "ZAR",
    at: "2026-11-01T08:00:00Z",
  }));
  const second = [{ seq: 50, transfer_id: "t50", direction: "OUT", amount_minor: "2500", asset: "ZAR", at: "2026-11-01T08:00:00Z" }];
  const afters: string[] = [];
  await page.route("**/api/v1/accounts/acc-zar/activity*", (route) => {
    const after = new URL(route.request().url()).searchParams.get("after") ?? "0";
    afters.push(after);
    return reply(route, 200, after === "0" ? { entries: first, next_after: 51 } : { entries: second, next_after: 0 });
  });

  await page.goto("/accounts/acc-zar");
  await expect(page.getByRole("row")).toHaveCount(51); // header plus 50 entries
  await page.getByRole("button", { name: "Load more" }).click();
  await expect(page.getByRole("row")).toHaveCount(52);
  await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0);
  expect(afters).toEqual(["0", "51"]);
});
