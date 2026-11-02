import { expect, test, type Page } from "@playwright/test";

const IDP = "http://127.0.0.1:3300";
const UPSTREAM = "http://127.0.0.1:3400";

// The development identity provider's users: two staff members with one role each, and a customer with no back-office role.
async function signInAs(page: Page, user: string) {
  await page.goto("/sign-in");
  await page.getByRole("link", { name: "Sign in with your identity provider" }).click();
  await page.waitForURL(`${IDP}/authorize**`);
  await page.getByRole("link", { name: user }).click();
}

test("the sign-in page sends staff to the identity provider, and no dev form is offered", async ({ page }) => {
  await page.goto("/sign-in");
  await expect(page.getByRole("link", { name: "Sign in with your identity provider" })).toBeVisible();
  await expect(page.getByLabel("Your name")).toHaveCount(0);
});

test("a compliance person signs in, sees their roles, and the API receives the token and no identity headers", async ({ page, request }) => {
  await signInAs(page, "Naledi Mokoena");
  await page.waitForURL("**/cases");
  await expect(page.getByTestId("staff-name")).toHaveText("Naledi Mokoena");
  await expect(page.getByTestId("staff-roles")).toHaveText("compliance");

  const last = await (await request.get(`${UPSTREAM}/__last`)).json();
  expect(last.path).toContain("/v1/cases");
  expect(last.authorization).toMatch(/^Bearer [A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/);
  expect(last.xForgeStaff).toBeNull();
  expect(last.xForgeRoles).toBeNull();
});

test("an operations person signs in with the operations role", async ({ page }) => {
  await signInAs(page, "Sipho Ndlovu");
  await page.waitForURL("**/cases");
  await expect(page.getByTestId("staff-roles")).toHaveText("operations");
});

test("a customer with no back-office role is sent back to sign-in with the reason, and keeps no session", async ({ page }) => {
  await signInAs(page, "Thandi Dube");
  await page.waitForURL("**/sign-in?error=*");
  await expect(page.locator('[data-testid="notice"][role="alert"]')).toHaveText("your account has no back-office role");
  await expect(page.getByRole("link", { name: "Sign in", exact: true })).toBeVisible();
});

test("a session cookie that is not a token the provider signed is no session", async ({ page, context }) => {
  await context.addCookies([{ name: "forge_staff_access", value: "not.a.token", url: "http://127.0.0.1:3201" }]);
  await page.goto("/cases");
  await expect(page.getByRole("link", { name: "Sign in", exact: true })).toBeVisible();
});

test("a callback whose state does not match is refused", async ({ page }) => {
  await page.goto("/api/auth/login");
  await page.goto("/api/auth/callback?code=anything&state=not-the-state");
  await page.waitForURL("**/sign-in?error=*");
  await expect(page.locator('[data-testid="notice"][role="alert"]')).toContainText("did not match");
});
