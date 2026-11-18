import { expect, test } from "@playwright/test";

const IDP = "http://127.0.0.1:3300";
const UPSTREAM = "http://127.0.0.1:3400";

// The browser signs in with the development identity provider: it asks the provider, picks a user, and comes back with a
// token the server has verified.
async function signInAs(page: import("@playwright/test").Page, user: string) {
  await page.goto("/sign-in");
  await page.getByRole("link", { name: "Sign in with your identity provider" }).click();
  await page.waitForURL(`${IDP}/authorize**`);
  await page.getByRole("link", { name: user }).click();
  await page.waitForURL("**/accounts");
}

test("the sign-in page sends the browser to the identity provider, and no dev form is offered", async ({ page }) => {
  await page.goto("/sign-in");
  await expect(page.getByRole("link", { name: "Sign in with your identity provider" })).toBeVisible();
  await expect(page.getByLabel("Your name")).toHaveCount(0);
});

test("signing in through the identity provider shows the person, and the API receives the token, not identity headers", async ({ page, request }) => {
  await signInAs(page, "Thandi Dube");
  await expect(page.getByTestId("signed-in-as")).toHaveText("Thandi Dube");

  const last = await (await request.get(`${UPSTREAM}/__last`)).json();
  expect(last.path).toContain("/v1/accounts");
  expect(last.authorization).toMatch(/^Bearer [A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/);
  expect(last.xForgeParty).toBeNull();
  expect(last.xForgeActor).toBeNull();
});

test("a session cookie that is not a token the provider signed is no session", async ({ page, context }) => {
  await context.addCookies([{ name: "forge_access", value: "not.a.token", url: "http://127.0.0.1:3101" }]);
  await page.goto("/accounts");
  await expect(page.getByRole("link", { name: "Sign in", exact: true })).toBeVisible();
});

test("a callback whose state does not match the sign-in that started it is refused", async ({ page }) => {
  await page.goto("/api/auth/login");
  await page.goto("/api/auth/callback?code=anything&state=not-the-state");
  await page.waitForURL("**/sign-in?error=*");
  await expect(page.getByTestId("notice")).toContainText("did not match");
});

test("signing out clears the session", async ({ page }) => {
  await signInAs(page, "Thandi Dube");
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("link", { name: "Sign in", exact: true })).toBeVisible();
});
