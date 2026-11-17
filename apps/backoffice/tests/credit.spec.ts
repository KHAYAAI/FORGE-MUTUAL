import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { reply, signIn } from "./helpers";

const app = {
  facility_id: "fac-11111111", product_id: "term-loan", status: "ASSESSED", requested_minor: "50000000", rate_bps: 1450,
  term_months: 12, fee_bps: 0, total_repayable_minor: "54010000", applied_at: "2026-11-17T09:00:00Z", purpose: "stock",
  party_id: "party-1",
  assessment: { inflow_90d_minor: "30000000", monthly_inflow_minor: "10000000", suggested_max_minor: "30000000", notes: ["The amount is more than the affordability rule suggests."] },
};

test("a credit officer approves less than was asked, with reasons", async ({ page }) => {
  let decision: unknown = null;
  let pending = [app];
  await page.route("**/api/v1/credit/applications", (route) => reply(route, 200, { facilities: pending }));
  await page.route("**/api/v1/credit/applications/*/decision", (route) => {
    decision = route.request().postDataJSON();
    pending = [];
    return reply(route, 200, { ...app, status: "APPROVED" });
  });
  await signIn(page, "Sipho", ["credit"]);
  await page.goto("/credit");
  const card = page.getByRole("listitem", { name: "Application fac-1111" });
  await expect(card.getByText("The amount is more than the affordability rule suggests.")).toBeVisible();
  const a11y = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(a11y.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => v.id)).toEqual([]);

  await expect(card.getByRole("button", { name: "Approve" })).toBeDisabled();
  await card.getByLabel(/Approved amount/).fill("300000");
  await card.getByLabel(/Your reasons/).fill("Inflows support R300 000; reduced from the amount asked.");
  await card.getByRole("button", { name: "Approve" }).click();
  await expect(page.getByText("Approved. The customer can now accept the offer.")).toBeVisible();
  expect(decision).toEqual({ approve: true, approved_minor: "30000000", note: "Inflows support R300 000; reduced from the amount asked." });
  await expect(page.getByText("No applications are waiting for a decision.")).toBeVisible();
});

test("staff without the credit role cannot decide", async ({ page }) => {
  await page.route("**/api/v1/credit/applications", (route) => reply(route, 200, { facilities: [app] }));
  await signIn(page, "Naledi", ["compliance"]);
  await page.goto("/credit");
  await expect(page.getByRole("listitem", { name: "Application fac-1111" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Approve" })).toHaveCount(0);
});
