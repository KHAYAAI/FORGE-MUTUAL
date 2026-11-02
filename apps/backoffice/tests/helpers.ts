import type { Page, Route } from "@playwright/test";
import type { components } from "../lib/api/schema";

type Case = components["schemas"]["Case"];

export const CASE_ID = "11111111-1111-4111-8111-111111111111";
export const PAYMENT_ID = "22222222-2222-4222-8222-222222222222";
export const BREAK_ID = "33333333-3333-4333-8333-333333333333";

export function reply(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

export function caseFixture(overrides: Partial<Case> = {}): Case {
  return {
    case_id: CASE_ID,
    party_id: "party-1",
    rule: "VELOCITY",
    severity: "CRITICAL",
    detail: "Outflow over the daily limit within a day.",
    evidence: ["txn-101", "txn-102"],
    status: "OPEN",
    opened_by: "system",
    opened_at: "2026-10-31T09:00:00Z",
    sla_due_at: "2026-11-01T09:00:00Z",
    overdue: false,
    ...overrides,
  };
}

/** Signs in through the real sign-in page, which sets the staff session cookie. */
export async function signIn(page: Page, name: string, roles: ("compliance" | "operations")[]) {
  await page.goto("/sign-in");
  await page.getByLabel("Your name").fill(name);
  for (const role of roles) {
    await page.locator(`input[name="${role}"]`).check();
  }
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/cases");
}
