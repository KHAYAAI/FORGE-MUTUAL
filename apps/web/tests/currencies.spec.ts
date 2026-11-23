import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { reply, signIn, usdAccount, zarAccount } from "./helpers";

const credit = {
  credit_id: "USD:mv-1", account_id: "acc-usd", asset: "USD", amount_minor: "250000", reference: "FORGEUSDPB1",
  booked_at: "2026-11-16T10:00:00Z", status: "AWAITING_REASON", reason_code: "", reason_note: "", classified_by: "", classified_at: "",
};

test("a customer opens a euro account and says what a dollar payment was for", async ({ page }) => {
  let accounts = [zarAccount, usdAccount];
  let opened: unknown = null;
  let reason: unknown = null;
  await page.route("**/api/v1/accounts", (route) => {
    if (route.request().method() === "POST") {
      opened = route.request().postDataJSON();
      accounts = [...accounts, { ...usdAccount, account_id: "acc-eur", asset: "EUR", available_minor: "0", holding_entity: "PARTNER BANK" }];
      return reply(route, 201, { account_id: "acc-eur", asset: "EUR", reference: "FORGEEURPB2", partner: "sim-partner-eur" });
    }
    return reply(route, 200, { accounts });
  });
  await page.route("**/api/v1/receiving-details", (route) => reply(route, 200, { details: [{ reference: "FORGEUSDPB1", asset: "USD", account_id: "acc-usd" }] }));
  await page.route("**/api/v1/payment-reasons", (route) =>
    reply(route, 200, { reasons: [{ code: "services_software", label: "Payment for software or IT services we provided" }] }),
  );
  await page.route("**/api/v1/cross-border-credits", (route) => reply(route, 200, { credits: reason ? [{ ...credit, status: "CLASSIFIED", reason_code: "services_software" }] : [credit] }));
  await page.route("**/api/v1/cross-border-credits/*/reason", (route) => {
    reason = route.request().postDataJSON();
    return reply(route, 200, { ...credit, status: "CLASSIFIED", reason_code: "services_software" });
  });

  await signIn(page);
  await page.getByRole("link", { name: "Currencies" }).click();
  await expect(page.getByRole("heading", { name: "Currencies" })).toBeVisible();
  await expect(page.getByText("not covered by deposit insurance")).toBeVisible();
  await expect(page.getByText("FORGEUSDPB1")).toBeVisible();

  const a11y = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
  expect(a11y.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => v.id)).toEqual([]);

  await page.getByRole("button", { name: "Open EUR account" }).click();
  await expect(page.getByText("Your EUR account is open.")).toBeVisible();
  expect(opened).toEqual({ asset: "EUR" });
  await expect(page.getByRole("button", { name: "Open EUR account" })).toHaveCount(0);

  const form = page.getByRole("listitem", { name: /Payment of US\$ 2 500.00/ });
  await form.getByRole("combobox").selectOption("services_software");
  await form.getByPlaceholder("Invoice number").fill("INV-2026-118");
  await form.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("The reason is recorded with the payment.")).toBeVisible();
  expect(reason).toEqual({ reason_code: "services_software", note: "INV-2026-118" });
  await expect(page.getByRole("heading", { name: /Tell us what/ })).toHaveCount(0);
});
