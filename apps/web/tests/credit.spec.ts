import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { reply, signIn, zarAccount } from "./helpers";

const products = [
  { product_id: "overdraft", name: "Overdraft facility", rate_bps: 1675, fee_bps: 0, sweep_bps: 0, min_minor: "500000", max_minor: "50000000", max_term_months: 0, available: true },
  { product_id: "term-loan", name: "Working-capital loan", rate_bps: 1450, fee_bps: 0, sweep_bps: 0, min_minor: "1000000", max_minor: "500000000", max_term_months: 60, available: true },
];
const base = {
  facility_id: "fac-1", product_id: "term-loan", requested_minor: "12000000", approved_minor: "12000000", rate_bps: 1450,
  term_months: 12, fee_bps: 0, sweep_bps: 0, outstanding_minor: "0", available_minor: "0", total_repayable_minor: "12950000",
  applied_at: "2026-11-17T09:00:00Z", schedule: [] as unknown[], days_past_due: 0,
};

test("a customer applies, sees the full cost, and accepts the offer", async ({ page }) => {
  let applied: unknown = null;
  let facilities: unknown[] = [];
  await page.route("**/api/v1/accounts", (route) => reply(route, 200, { accounts: [zarAccount] }));
  await page.route("**/api/v1/loan-products", (route) => reply(route, 200, { products }));
  await page.route("**/api/v1/facilities", (route) => {
    if (route.request().method() === "POST") {
      applied = route.request().postDataJSON();
      facilities = [{ ...base, status: "ASSESSED", approved_minor: "0" }];
      return reply(route, 201, facilities[0]);
    }
    return reply(route, 200, { facilities });
  });
  await page.route("**/api/v1/facilities/fac-1/accept", (route) => {
    facilities = [{ ...base, status: "ACTIVE", outstanding_minor: "12000000", schedule: [{ number: 1, due_on: "2026-12-17", principal_minor: "934000", interest_minor: "145000", status: "DUE" }] }];
    return reply(route, 200, facilities[0]);
  });

  await signIn(page);
  await page.getByRole("link", { name: "Credit", exact: true }).click();
  const form = page.getByRole("form", { name: "Apply for credit" });
  await form.getByLabel("Product").selectOption("term-loan");
  await form.getByLabel("Amount (R)").fill("120000");
  await form.getByLabel("Term (months)").fill("12");
  await form.getByLabel("What is it for?").fill("stock");
  await form.getByRole("button", { name: "Apply" }).click();
  await expect(page.getByText("A credit officer will review your application.")).toBeVisible();
  expect(applied).toMatchObject({ product_id: "term-loan", amount_minor: "12000000", term_months: 12, current_account_id: "acc-zar" });

  facilities = [{ ...base, status: "APPROVED" }];
  await page.reload();
  const card = page.getByRole("listitem", { name: "Working-capital loan" });
  await expect(card.getByText("R 129 500.00")).toBeVisible();
  const a11y = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(a11y.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => v.id)).toEqual([]);
  await card.getByRole("button", { name: "Accept the offer" }).click();
  await expect(page.getByText("Accepted. The facility is active.")).toBeVisible();
  await expect(card.getByText("Next payment R 10 790.00 on 2026-12-17")).toBeVisible();
});
