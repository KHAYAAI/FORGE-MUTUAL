import { expect, test } from "@playwright/test";
import { reply, signIn } from "./helpers";

test("an unsigned visitor is sent to sign in", async ({ page }) => {
  await page.route("**/api/v1/cases?*", (route) => reply(route, 401, { error: { code: "unauthenticated", message: "sign in to continue" } }));
  await page.goto("/cases");
  await page.waitForURL("**/sign-in");
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
});

test("sign-in refuses a person who holds no role", async ({ page }) => {
  await page.goto("/sign-in");
  await page.getByLabel("Your name").fill("Naledi");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.locator('[data-testid="notice"][role="alert"]')).toHaveText("enter your name and choose at least one role");
  await expect(page).toHaveURL(/\/sign-in$/);
});

test("the nav shows the signed-in name and roles", async ({ page }) => {
  await page.route("**/api/v1/cases?*", (route) => reply(route, 200, { cases: [] }));
  await signIn(page, "Naledi", ["compliance", "operations"]);
  await expect(page.getByTestId("staff-name")).toHaveText("Naledi");
  await expect(page.getByTestId("staff-roles")).toHaveText("compliance, operations");
});

// No API route is mocked here. The proxy points at a closed port, so this exercises its unreachable path for real.
test("when the back office service is unreachable, the page says so", async ({ page }) => {
  await signIn(page, "Naledi", ["compliance"]);
  await expect(page.getByTestId("notice")).toHaveText("the back office service is not reachable; try again shortly");
});
