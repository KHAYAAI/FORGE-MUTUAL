import { randomUUID } from "node:crypto";
import { expect, test, type Page } from "@playwright/test";
import { BACKOFFICE, send, startCompany, verdict, waitForAccounts, waitForState } from "../../../../deploy/compose/journeys/seed.cjs";

// The four products added for launch, against the real stack: currency accounts, credit, savings and card payments.
// Each runs on its real service, with the real ledger. The only outside parties are the identity provider's verdict
// (delivered signed, as Didit would) and the acquirer, which is the simulator.

async function signInAs(page: Page, partyId: string) {
  await page.goto("/sign-in");
  await page.getByLabel("Party ID").fill(partyId);
  await page.getByLabel("Your name").fill("Thandi");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/accounts");
}

const json = { "content-type": "application/json" };
const keyed = () => ({ ...json, "idempotency-key": randomUUID() });

test("a business opens a euro account, borrows, saves, and takes a card payment", async ({ page }) => {
  test.setTimeout(240_000);
  const company = await startCompany("products");
  await verdict(company.onboardingId, company.companyPartyId, "APPROVED");
  await verdict(company.onboardingId, company.personPartyIds[0], "APPROVED");
  await waitForState(company.onboardingId, "ONBOARDING_STATE_COMPLETED");
  const [current] = await waitForAccounts(company.companyPartyId);
  await signInAs(page, company.companyPartyId);
  const api = page.request;

  // A euro account, opened once at the partner bank, with a reference payers quote.
  const eur = await api.post("/api/v1/accounts", { headers: json, data: { asset: "EUR" } });
  expect(eur.status(), await eur.text()).toBe(201);
  const opened = await eur.json();
  expect(opened.reference).toMatch(/^FORGEEUR/);
  const again = await (await api.post("/api/v1/accounts", { headers: json, data: { asset: "EUR" } })).json();
  expect(again.account_id).toBe(opened.account_id);
  const details = await (await api.get("/api/v1/receiving-details")).json();
  expect(details.details.map((d: { reference: string }) => d.reference)).toContain(opened.reference);

  // Credit: apply, a credit officer decides in the back office, the customer accepts, and the money arrives.
  const applied = await api.post("/api/v1/facilities", {
    headers: keyed(),
    data: { product_id: "term-loan", amount_minor: "5000000", term_months: 6, current_account_id: current.account_id, purpose: "launch stock" },
  });
  expect(applied.status(), await applied.text()).toBe(201);
  const facility = await applied.json();
  expect(facility.status).toBe("ASSESSED");
  const decided = await send(`${BACKOFFICE}/v1/credit/applications/${facility.facility_id}/decision`, {
    method: "POST",
    headers: { ...json, "x-forge-staff": "Sipho", "x-forge-roles": "credit" },
    body: JSON.stringify({ approve: true, note: "New customer; approved at the requested amount for the stack journey." }),
  });
  expect(decided.status, decided.text).toBe(200);
  const accepted = await (await api.post(`/api/v1/facilities/${facility.facility_id}/accept`, { headers: json, data: { accepts_terms: true } })).json();
  expect(accepted.status).toBe("ACTIVE");
  expect(accepted.schedule).toHaveLength(6);
  await expect
    .poll(async () => {
      const accs = (await (await api.get("/api/v1/accounts")).json()).accounts as { account_id: string; available_minor: string }[];
      return accs.find((a) => a.account_id === current.account_id)?.available_minor;
    }, { timeout: 60_000 })
    .toBe("5000000");

  // Savings: open, move money in, take some out at once.
  const sav = await api.post("/api/v1/deposit-accounts", { headers: json, data: { product_id: "savings" } });
  expect(sav.status(), await sav.text()).toBe(201);
  const savings = await sav.json();
  const dep = await api.post("/api/v1/deposits", { headers: keyed(), data: { from_account_id: current.account_id, to_account_id: savings.account_id, amount_minor: "2000000" } });
  expect(dep.status(), await dep.text()).toBe(200);
  const wd = await api.post("/api/v1/withdrawals", { headers: keyed(), data: { from_account_id: savings.account_id, to_account_id: current.account_id, amount_minor: "500000" } });
  expect((await wd.json()).state).toBe("MOVED");
  const list = (await (await api.get("/api/v1/deposit-accounts")).json()).accounts;
  expect(list[0].balance_minor).toBe("1500000");

  // Card payments: a link, paid through the acquirer simulator's checkout.
  const created = await api.post("/api/v1/payment-links", {
    headers: keyed(),
    data: { settlement_account_id: current.account_id, amount_minor: "120000", description: "Workshop seat", merchant_name: "Journey Studio" },
  });
  expect(created.status(), await created.text()).toBe(201);
  const link = await created.json();
  const payer = await page.context().browser()!.newContext({ baseURL: new URL(page.url()).origin });
  const p = await payer.newPage();
  await p.goto(`/pay/${link.token}`);
  await expect(p.getByText("Pay Journey Studio")).toBeVisible();
  await p.getByRole("button", { name: "Pay by card" }).click();
  await p.getByRole("button", { name: "Approve test payment" }).click();
  await expect(p.getByText("Paid. Thank you.")).toBeVisible({ timeout: 15_000 });
  const links = (await (await api.get("/api/v1/payment-links")).json()).links;
  expect(links.find((l: { link_id: string }) => l.link_id === link.link_id).status).toBe("PAID");
  await payer.close();
});
