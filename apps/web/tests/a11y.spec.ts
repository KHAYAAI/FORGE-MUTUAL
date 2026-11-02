import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { mockAccounts, signIn } from "./helpers";

// Accessibility journey: every customer page is scanned against WCAG 2.1 A and AA with axe. Serious and critical
// violations fail the build; minor ones are reported in the test output for the team to triage.
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

test("the sign-in page is accessible", async ({ page }) => {
  await page.goto("/sign-in");
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
  await expectAccessible(page, "sign-in");
});

test("the signed-in pages are accessible", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);
  await expectAccessible(page, "accounts");
  for (const [path, label] of [
    ["/fx", "fx"],
    ["/payments/new", "new payment"],
    ["/payouts/new", "new payout"],
  ] as const) {
    await page.goto(path);
    await expect(page.locator("main, h1").first()).toBeVisible();
    await expectAccessible(page, label);
  }
});
