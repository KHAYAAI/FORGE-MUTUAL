import { expect, test } from "@playwright/test";
import { reply, signIn } from "./helpers";

// A sign-in that did not use a second factor is sent back to sign in with a plain explanation, not left on a page of
// errors.
test("a sign-in without a second factor is told what to do", async ({ page }) => {
  await signIn(page);
  await page.route("**/api/v1/**", (route) =>
    reply(route, 403, { error: { code: "mfa_required", message: "your sign-in did not use a second factor" } }),
  );
  // The redirect can land before the page finishes loading, so wait only for the navigation to start.
  await page.goto("/currencies", { waitUntil: "commit" });
  await expect(page).toHaveURL(/\/sign-in\?error=/, { timeout: 10_000 });
  await expect(page.getByText("did not use a second factor, which every Forge account requires")).toBeVisible();
});
