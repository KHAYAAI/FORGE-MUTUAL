import { expect, test } from "@playwright/test";
import { mockAccounts, reply, signIn } from "./helpers";

const BATCH = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee";

const batch = (state: string, approvedBy = "") => ({
  batch_id: BATCH,
  from_account_id: "acc-zar",
  asset: "ZAR",
  total_minor: "150000",
  state,
  created_by: "Thandi",
  approved_by: approvedBy,
  run: state === "BATCH_STATE_DRAFT" ? 0 : 1,
  items: [
    { item_key: "p1", amount_minor: "100000", state: "ITEM_STATE_PENDING" },
    { item_key: "p2", amount_minor: "50000", state: "ITEM_STATE_FAILED", reason: "invalid_argument: bad account" },
  ],
});

test("the maker creates a batch, which waits for approval", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);
  let created: { key?: string; body?: Record<string, unknown> } = {};
  await page.route("**/api/v1/payout-batches", (route) => {
    created = {
      key: route.request().headers()["idempotency-key"],
      body: JSON.parse(route.request().postData() ?? "{}"),
    };
    return reply(route, 201, batch("BATCH_STATE_DRAFT"));
  });
  await page.route(`**/api/v1/payout-batches/${BATCH}`, (route) => reply(route, 200, batch("BATCH_STATE_DRAFT")));

  await page.goto("/payouts/new");
  await page.getByLabel("Amount 1").fill("1000");
  await page.getByLabel("Beneficiary 1").fill("Thandi Dube");
  await page.getByLabel("Account number 1").fill("1234567890");
  await page.getByLabel("Bank code 1").fill("250655");
  await page.getByLabel("Reference 1").fill("invoice 42");
  await page.getByRole("button", { name: "Create batch" }).click();

  await page.waitForURL(`**/payouts/${BATCH}`);
  await expect(page.getByTestId("batch-state")).toHaveText("Waiting for approval");
  expect(created.key).toBeTruthy();
  expect(created.body).toMatchObject({
    from_account_id: "acc-zar",
    asset: "ZAR",
    items: [{ item_key: "p1", amount_minor: "100000", beneficiary_name: "Thandi Dube" }],
  });
});

test("the checker approves, and the batch moves to sending with its failed items shown", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);
  await page.route(`**/api/v1/payout-batches/${BATCH}`, (route) => reply(route, 200, batch("BATCH_STATE_DRAFT")));
  let approveCalls = 0;
  await page.route(`**/api/v1/payout-batches/${BATCH}/approve`, (route) => {
    approveCalls += 1;
    return reply(route, 200, batch("BATCH_STATE_RUNNING", "Sipho"));
  });

  await page.goto(`/payouts/${BATCH}`);
  await page.getByRole("button", { name: "Approve and send" }).click();

  await expect(page.getByTestId("batch-state")).toHaveText("Sending");
  await expect(page.getByText("Failed")).toBeVisible();
  await expect(page.getByText("invalid_argument: bad account")).toBeVisible();
  expect(approveCalls).toBe(1);
});

test("the maker cannot approve their own batch, and is told why", async ({ page }) => {
  await mockAccounts(page);
  await signIn(page);
  await page.route(`**/api/v1/payout-batches/${BATCH}`, (route) => reply(route, 200, batch("BATCH_STATE_DRAFT")));
  await page.route(`**/api/v1/payout-batches/${BATCH}/approve`, (route) =>
    reply(route, 403, { error: { code: "permission_denied", message: "the checker must not be the maker" } }),
  );

  await page.goto(`/payouts/${BATCH}`);
  await page.getByRole("button", { name: "Approve and send" }).click();
  await expect(page.getByTestId("notice")).toHaveText("the checker must not be the maker");
  await expect(page.getByTestId("batch-state")).toHaveText("Waiting for approval");
});
