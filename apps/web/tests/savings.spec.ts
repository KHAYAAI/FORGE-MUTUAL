import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { reply, signIn, zarAccount } from "./helpers";

const products = [
  { product_id: "savings", name: "Savings", rate_bps: 650, notice_days: 0, available: true },
  { product_id: "notice-32", name: "32-day notice", rate_bps: 775, notice_days: 32, available: true },
  { product_id: "notice-90", name: "90-day notice", rate_bps: 850, notice_days: 90, available: false },
];
const notice32 = {
  account_id: "acc-n32", product_id: "notice-32", rate_bps: 775, notice_days: 32, balance_minor: "3000000",
  accrued_minor: "1234", under_notice_minor: "0", available_minor: "3000000", opened_at: "2026-10-01T09:00:00Z",
  interest_paid_minor: "55205", notices: [] as unknown[],
};

test("a customer gives notice on a notice account; an unapproved product cannot be opened", async ({ page }) => {
  let withdrawal: { body: unknown; key: string | null } | null = null;
  let accounts = [notice32];
  await page.route("**/api/v1/accounts", (route) => reply(route, 200, { accounts: [zarAccount, { ...zarAccount, account_id: "acc-n32" }] }));
  await page.route("**/api/v1/deposit-products", (route) => reply(route, 200, { products }));
  await page.route("**/api/v1/deposit-accounts", (route) => reply(route, 200, { accounts }));
  await page.route("**/api/v1/withdrawals", (route) => {
    withdrawal = { body: route.request().postDataJSON(), key: route.request().headers()["idempotency-key"] ?? null };
    const notice = { notice_id: "n-1", amount_minor: "1000000", to_account_id: "acc-zar", given_at: "2026-11-17T10:00:00Z", due_on: "2026-12-19", status: "PENDING" };
    accounts = [{ ...notice32, under_notice_minor: "1000000", available_minor: "2000000", notices: [notice] }];
    return reply(route, 200, { state: "NOTICE_GIVEN", notice });
  });

  await signIn(page);
  await page.getByRole("link", { name: "Savings" }).click();
  await expect(page.getByRole("heading", { name: "Savings", level: 1 })).toBeVisible();
  const card = page.getByRole("listitem", { name: "32-day notice" });
  await expect(card.getByTestId("savings-balance")).toHaveText("R 30 000.00");
  await expect(card.getByText("R 552.05")).toBeVisible();
  await expect(page.getByRole("button", { name: "Open 90-day notice" })).toHaveCount(0);
  await expect(page.getByText("Not yet available")).toBeVisible();

  const a11y = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(a11y.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => v.id)).toEqual([]);

  await card.getByLabel("Amount (R)").fill("10000");
  await card.getByRole("button", { name: "Give notice" }).click();
  await expect(page.getByText("Notice given. The money will be paid on 2026-12-19.")).toBeVisible();
  expect(withdrawal!.body).toEqual({ from_account_id: "acc-n32", to_account_id: "acc-zar", amount_minor: "1000000" });
  expect(withdrawal!.key).toMatch(/^[0-9a-f-]{36}$/);
  await expect(card.getByText("R 10 000.00 paid on 2026-12-19")).toBeVisible();
});
