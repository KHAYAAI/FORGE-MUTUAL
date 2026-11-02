import { expect, test } from "@playwright/test";
import { PAYMENT_ID, reply, signIn } from "./helpers";

const payment = (state: string) => ({
  payment_id: PAYMENT_ID,
  state,
  amount_minor: "1250000",
  asset: "ZAR",
  initiated_by: "Thandi",
  reason: "",
});

test("an operations person approves a payment that is awaiting approval, and the decision names them", async ({ page }) => {
  let state = "PAYMENT_STATE_AWAITING_APPROVAL";
  await page.route(`**/api/v1/payments/${PAYMENT_ID}`, (route) => reply(route, 200, payment(state)));
  let decided: { approve?: boolean } | null = null;
  await page.route(`**/api/v1/payments/${PAYMENT_ID}/approval`, (route) => {
    decided = JSON.parse(route.request().postData() ?? "{}");
    state = "PAYMENT_STATE_RUNNING";
    return reply(route, 200, { payment_id: PAYMENT_ID, decision: "APPROVED" });
  });

  await signIn(page, "Sipho", ["operations"]);
  await page.goto(`/payments/${PAYMENT_ID}`);
  await expect(page.getByTestId("payment-state")).toHaveText("Awaiting approval");
  await expect(page.getByText("R 12 500.00")).toBeVisible();
  await page.getByRole("button", { name: "Approve" }).click();

  await expect(page.getByTestId("notice")).toHaveText("Payment approved.");
  expect(decided).toEqual({ approve: true });
});

test("a compliance person sees the payment but not the decision buttons", async ({ page }) => {
  await page.route(`**/api/v1/payments/${PAYMENT_ID}`, (route) =>
    reply(route, 200, payment("PAYMENT_STATE_AWAITING_APPROVAL")),
  );
  await signIn(page, "Naledi", ["compliance"]);
  await page.goto(`/payments/${PAYMENT_ID}`);
  await expect(page.getByText("An operations role is needed to decide this payment.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Approve" })).toHaveCount(0);
});

test("a repair records what the rail confirms", async ({ page }) => {
  let state = "PAYMENT_STATE_REPAIR_REQUIRED";
  await page.route(`**/api/v1/payments/${PAYMENT_ID}`, (route) => reply(route, 200, payment(state)));
  let repaired: { settled?: boolean } | null = null;
  await page.route(`**/api/v1/payments/${PAYMENT_ID}/repair`, (route) => {
    repaired = JSON.parse(route.request().postData() ?? "{}");
    state = "PAYMENT_STATE_COMPLETED";
    return reply(route, 200, { payment_id: PAYMENT_ID, outcome: "SETTLED" });
  });

  await signIn(page, "Sipho", ["operations"]);
  await page.goto(`/payments/${PAYMENT_ID}`);
  await expect(page.getByTestId("payment-state")).toHaveText("Repair required: the rail's outcome is unknown");
  await page.getByRole("button", { name: "The rail confirms the money moved" }).click();

  await expect(page.getByTestId("payment-state")).toHaveText("Completed");
  expect(repaired).toEqual({ settled: true });
});

test("a decision the server refuses is shown, and the payment does not change", async ({ page }) => {
  await page.route(`**/api/v1/payments/${PAYMENT_ID}`, (route) =>
    reply(route, 200, payment("PAYMENT_STATE_AWAITING_APPROVAL")),
  );
  await page.route(`**/api/v1/payments/${PAYMENT_ID}/approval`, (route) =>
    reply(route, 409, { error: { code: "failed_precondition", message: "the payment is not awaiting approval" } }),
  );
  await signIn(page, "Sipho", ["operations"]);
  await page.goto(`/payments/${PAYMENT_ID}`);
  await page.getByRole("button", { name: "Approve" }).click();
  await expect(page.getByTestId("notice")).toHaveText("the payment is not awaiting approval");
  await expect(page.getByTestId("payment-state")).toHaveText("Awaiting approval");
});
