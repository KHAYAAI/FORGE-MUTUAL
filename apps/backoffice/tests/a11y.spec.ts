import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { reply, signIn } from "./helpers";

// Accessibility journey for the staff screens: every page is scanned against WCAG 2.1 A and AA with axe. Serious and
// critical violations fail the build; minor ones are reported in the test output for triage.
async function expectAccessible(page: Page, label: string) {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  const blocking = results.violations.filter((v) => v.impact === "serious" || v.impact === "critical");
  const minor = results.violations.filter((v) => v.impact !== "serious" && v.impact !== "critical");
  for (const v of minor) {
    console.log(`[a11y minor] ${label}: ${v.id} (${v.impact}) ${v.nodes.length} node(s): ${v.help}`);
  }
  expect(
    blocking.map((v) => `${v.id} (${v.impact}): ${v.help} — ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`),
    `${label} has serious or critical accessibility violations`,
  ).toEqual([]);
}

test("the staff sign-in page is accessible", async ({ page }) => {
  await page.goto("/sign-in");
  await expect(page.getByRole("heading", { name: "Staff sign-in" })).toBeVisible();
  await expectAccessible(page, "sign-in");
});

test("the staff pages are accessible", async ({ page }) => {
  await page.route("**/api/v1/**", (route) => reply(route, 200, { cases: [], reviews: [] }));
  await signIn(page, "Naledi", ["compliance", "operations"]);
  for (const [path, label] of [
    ["/cases", "cases"],
    ["/kyc", "kyc reviews"],
    ["/recon", "reconciliation"],
    ["/payments/lookup", "payment lookup"],
  ] as const) {
    await page.goto(path);
    await expect(page.locator("main, h1").first()).toBeVisible();
    await expectAccessible(page, label);
  }
});
