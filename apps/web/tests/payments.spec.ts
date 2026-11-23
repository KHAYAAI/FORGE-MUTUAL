import { expect, test } from "@playwright/test";
import { mockAccounts, reply, signIn } from "./helpers";

async function fillPayment(page: import("@playwright/test").Page, amount: string) {
  await page.getByLabel("Amount (R)").fill(amount);
  await page.getByLabel("Beneficiary name").fill("Thandi Dube");
  await page.getByLabel("Account number").fill("1234567890");
  await page.getByLabel("Bank code").fill("250655");
  await page.getByLabel("Reference").fill("invoice 42");
}

test("a payment is sent once, with an idempotency key and the amount in minor units", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);

  const requests: { key: string | undefined; body: Record<string, unknown> }[] = [];
  await page.route("**/api/v1/payments", (route) => {
    const req = route.request();
    requests.push({ key: req.headers()["idempotency-key"], body: JSON.parse(req.postData() ?? "{}") });
    return reply(route, 202, { payment_id: "11111111-2222-4333-8444-555555555555" });
  });

  await page.goto("/payments/new");
  await fillPayment(page, "125.50");
  await page.getByRole("button", { name: "Send payment" }).click();

  await expect(page.getByTestId("notice")).toContainText("Payment accepted");
  await expect(page.getByRole("link", { name: "See its status" })).toBeVisible();
  expect(requests).toHaveLength(1);
  expect(requests[0].key).toBeTruthy();
  expect(requests[0].body).toMatchObject({ account_id: "acc-zar", amount_minor: "12550", asset: "ZAR" });
});

test("a refused payment shows the reason the API gives", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);
  await page.route("**/api/v1/payments", (route) =>
    reply(route, 403, { error: { code: "permission_denied", message: "the account does not belong to you" } }),
  );

  await page.goto("/payments/new");
  await fillPayment(page, "10");
  await page.getByRole("button", { name: "Send payment" }).click();
  await expect(page.getByTestId("notice")).toHaveText("the account does not belong to you");
});

test("an amount that is not a positive value with at most two decimals is refused before anything is sent", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);
  let sent = false;
  await page.route("**/api/v1/payments", (route) => {
    sent = true;
    return reply(route, 202, { payment_id: "x" });
  });

  await page.goto("/payments/new");
  for (const bad of ["0", "-5", "1.234", "abc"]) {
    await fillPayment(page, bad);
    await page.getByRole("button", { name: "Send payment" }).click();
    await expect(page.getByTestId("notice")).toContainText("Enter an amount greater than zero");
  }
  expect(sent).toBe(false);
});
