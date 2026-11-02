import { expect, test } from "@playwright/test";
import { BREAK_ID, reply, signIn } from "./helpers";

const gateClosed = { break_active: true, reasons: ["break open: cash, ZAR"], as_of: "2026-11-01T08:00:00Z" };
const openBreak = {
  break_id: BREAK_ID,
  asset: "ZAR",
  check: "cash",
  difference_minor: "25000",
  detail: "Statement closing differs from the ledger by R 250.00.",
  status: "OPEN",
  opened_at: "2026-11-01T06:00:00Z",
  last_seen_at: "2026-11-01T08:00:00Z",
  investigation_note: "",
  investigated_by: "",
};

test("a closed gate shows its reasons and says that payments are held", async ({ page }) => {
  await page.route("**/api/v1/recon/gate", (route) => reply(route, 200, gateClosed));
  await page.route("**/api/v1/recon/breaks?*", (route) => reply(route, 200, { breaks: [openBreak] }));
  await signIn(page, "Sipho", ["operations"]);
  await page.goto("/recon");
  await expect(page.getByTestId("gate")).toContainText("The payments gate is closed. Payments are held.");
  await expect(page.getByTestId("gate")).toContainText("break open: cash, ZAR");
});

test("an investigation is recorded, and the break stays open", async ({ page }) => {
  await page.route("**/api/v1/recon/gate", (route) => reply(route, 200, gateClosed));
  await page.route("**/api/v1/recon/breaks?*", (route) => reply(route, 200, { breaks: [openBreak] }));
  let recorded: { note?: string } | null = null;
  await page.route(`**/api/v1/recon/breaks/${BREAK_ID}/investigate`, (route) => {
    recorded = JSON.parse(route.request().postData() ?? "{}");
    return reply(route, 200, {
      ...openBreak,
      investigation_note: recorded?.note ?? "",
      investigated_by: "Sipho",
      investigated_at: "2026-11-01T08:30:00Z",
    });
  });

  await signIn(page, "Sipho", ["operations"]);
  await page.goto("/recon");
  await page.getByLabel("What did you find?").fill("The feed was re-sent after a timeout; the duplicate was reversed.");
  await page.getByRole("button", { name: "Record investigation" }).click();

  await expect(page.getByText("Investigation by Sipho:")).toBeVisible();
  await expect(page.getByTestId("break-status")).toHaveText("Open");
  expect(recorded).toEqual({ note: "The feed was re-sent after a timeout; the duplicate was reversed." });
});

test("a compliance person sees breaks but cannot investigate them", async ({ page }) => {
  await page.route("**/api/v1/recon/gate", (route) => reply(route, 200, gateClosed));
  await page.route("**/api/v1/recon/breaks?*", (route) => reply(route, 200, { breaks: [openBreak] }));
  await signIn(page, "Naledi", ["compliance"]);
  await page.goto("/recon");
  await expect(page.getByTestId("break-status")).toHaveText("Open");
  await expect(page.getByRole("button", { name: "Record investigation" })).toHaveCount(0);
});
