import { expect, test } from "@playwright/test";
import { CASE_ID, caseFixture, reply, signIn } from "./helpers";

test("a case past its SLA is marked overdue on the list", async ({ page }) => {
  await page.route("**/api/v1/cases?*", (route) => reply(route, 200, { cases: [caseFixture({ overdue: true })] }));
  await signIn(page, "Naledi", ["compliance"]);
  await expect(page.getByTestId("overdue")).toHaveText("Overdue");
});

test("a compliance person closes a case with a note of at least twenty characters", async ({ page }) => {
  await page.route(`**/api/v1/cases/${CASE_ID}`, (route) => reply(route, 200, caseFixture()));
  let closed: { note?: string } | null = null;
  await page.route(`**/api/v1/cases/${CASE_ID}/close`, (route) => {
    closed = JSON.parse(route.request().postData() ?? "{}");
    return reply(route, 200, caseFixture({ status: "CLOSED", closed_by: "Naledi", closing_note: closed?.note ?? "" }));
  });

  await signIn(page, "Naledi", ["compliance"]);
  await page.goto(`/cases/${CASE_ID}`);

  const button = page.getByRole("button", { name: "Close case" });
  await page.getByLabel(/Closing note/).fill("too short");
  await expect(button).toBeDisabled();

  await page.getByLabel(/Closing note/).fill("Reviewed against the bank statement; a documented payroll run.");
  await expect(button).toBeEnabled();
  await button.click();

  await expect(page.getByTestId("case-status")).toHaveText("Closed");
  expect(closed).toEqual({ note: "Reviewed against the bank statement; a documented payroll run." });
});

test("the person who opened a case is told another must close it, and is not offered the button", async ({ page }) => {
  await page.route(`**/api/v1/cases/${CASE_ID}`, (route) => reply(route, 200, caseFixture({ opened_by: "Naledi" })));
  await signIn(page, "Naledi", ["compliance"]);
  await page.goto(`/cases/${CASE_ID}`);
  await expect(page.getByText("You opened this case. Another compliance person must close it.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Close case" })).toHaveCount(0);
});

test("an operations person sees the case but cannot close it", async ({ page }) => {
  await page.route(`**/api/v1/cases/${CASE_ID}`, (route) => reply(route, 200, caseFixture()));
  await signIn(page, "Sipho", ["operations"]);
  await page.goto(`/cases/${CASE_ID}`);
  await expect(page.getByText("Only a compliance role can close a case.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Close case" })).toHaveCount(0);
});

test("a close the server refuses shows the server's reason", async ({ page }) => {
  await page.route(`**/api/v1/cases/${CASE_ID}`, (route) => reply(route, 200, caseFixture()));
  await page.route(`**/api/v1/cases/${CASE_ID}/close`, (route) =>
    reply(route, 403, { error: { code: "permission_denied", message: "the person who opened a case cannot close it" } }),
  );
  await signIn(page, "Naledi", ["compliance"]);
  await page.goto(`/cases/${CASE_ID}`);
  await page.getByLabel(/Closing note/).fill("Reviewed against the bank statement; a documented payroll run.");
  await page.getByRole("button", { name: "Close case" }).click();
  await expect(page.getByTestId("notice")).toHaveText("the person who opened a case cannot close it");
});
