import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { reply, signIn } from "./helpers";

test("the risk page shows each ratio against its minimum, the stress results, and its own limits", async ({ page }) => {
  await page.route("**/api/v1/risk/prudential", (route) =>
    reply(route, 200, {
      as_of: "2026-11-17T16:00:00Z", capital_minor: "0", total_rwa_minor: "570000000", credit_rwa_minor: "570000000", market_rwa_minor: "0",
      operational_rwa_minor: "0", total_capital_bps: "0", min_total_capital_bps: "1150", leverage_bps: "0", min_leverage_bps: "400",
      hqla_minor: "800000000", net_outflows_30d_minor: "280000000", lcr_bps: "28571", min_lcr_bps: "10000",
      deposits_minor: "1000000000", loans_minor: "400000000", largest_depositor_minor: "150000000",
      breaches: ["TOTAL_CAPITAL_RATIO", "LEVERAGE_RATIO"],
      stress: [{ name: "RUN_SEVERE", description: "Twice the standard run-off rates: a loss of confidence", liquidity_left_minor: "200000000", total_capital_bps: "0", survives: false }],
      limitations: ["Capital and gross income are configured figures, not read from the ledger (no equity accounts yet)."],
    }),
  );
  await page.route("**/api/v1/risk/trial-balance", (route) =>
    reply(route, 200, {
      as_of: "2026-11-17T16:00:00Z", total_assets_minor: "330000", total_liabilities_minor: "329000", internal_minor: "1000",
      checks: [{ asset: "USD", net_minor: "0", balanced: true }, { asset: "ZAR", net_minor: "0", balanced: true }],
      balance_sheet: [
        { section: "ASSETS", label: "Cash at the sponsor and partner banks", rand_minor: "280000" },
        { section: "LIABILITIES", label: "Customer deposits", rand_minor: "149000" },
        { section: "INTERNAL", label: "FEES_INCOME", rand_minor: "1000" },
      ],
    }),
  );
  await signIn(page, "Lerato", ["operations"]);
  await page.goto("/risk");
  await expect(page.getByText("In breach: Total capital ratio, Leverage ratio.")).toBeVisible();
  await expect(page.getByText("285.71%")).toBeVisible();
  await expect(page.getByText("Fails")).toBeVisible();
  await expect(page.getByText("no equity accounts yet")).toBeVisible();
  await expect(page.getByText("Every ledger balances: debits equal credits in USD, ZAR.")).toBeVisible();
  await expect(page.getByRole("row", { name: /Assets less liabilities/ })).toContainText("R 10.00");
  const a11y = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(a11y.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => v.id)).toEqual([]);
});
