// Records a guided demo of the customer workspace (apps/web) with the workspace API mocked in the browser.
// Every figure shown is sample data. Usage: start `next start -p 3100` with FORGE_AUTH_MODE=dev, then
//   node scripts/demo-video.mjs <out-dir>
import { createRequire } from "node:module";
const { chromium } = createRequire(import.meta.url)("playwright");

const BASE = "http://127.0.0.1:3100";
const out = process.argv[2] ?? "demo-out";
const W = 1280, H = 720;
const ok = (r, b, s = 200) => r.fulfill({ status: s, contentType: "application/json", body: JSON.stringify(b) });

const acct = (id, asset, minor, entity, insured, product) =>
  ({ account_id: id, asset, available_minor: minor, holding_entity: entity, insured, product_id: product, as_of: "2026-11-25T08:00:00Z", source: "LEDGER" });
const accounts = [acct("acc-zar", "ZAR", "184562050", "SPONSOR BANK", true, "current"), acct("acc-usd", "USD", "4250000", "SPONSOR BANK", false, "usd")];
let cards = [];
const card = { card_id: "card-1", party_id: "p", account_id: "acc-zar", label: "Cloud and tools", last4: "4242", expiry_month: 3, expiry_year: 2030,
  status: "ACTIVE", per_transaction_limit_minor: "1000000", daily_limit_minor: "2500000", created_at: "2026-11-25T10:00:00Z", network: "SIMULATED" };

async function mocks(page) {
  await page.route("**/api/v1/**", (r) => ok(r, { error: { code: "not_found", message: "not offered in this demo" } }, 404));
  await page.route("**/api/v1/accounts", (r) => ok(r, { accounts }));
  await page.route("**/api/v1/fx/quotes", (r) => ok(r, {
    quote_id: "q-1", sell_asset: "ZAR", buy_asset: "USD", sell_amount_minor: "5000000", buy_amount_minor: "275482",
    rate_scaled: "1815000000", mid_rate_scaled: "1804000000", spread_bps: 60, spread_minor: "29900",
    expires_at: new Date(Date.now() + 120000).toISOString(),
    disclosure: "You pay R 50 000.00. You receive US$ 2 754.82 at 18.15 ZAR per USD, which includes our spread of R 299.00." }));
  await page.route("**/api/v1/fx/deals", (r) => ok(r, { deal_id: "deal-2041", state: "BOOKED" }, 202));
  await page.route("**/api/v1/payments", (r) => ok(r, { payment_id: "7d1c2b9e-4a55-4c1e-9b0a-2f6e8c3d1a77" }, 202));
  await page.route("**/api/v1/deposit-products", (r) => ok(r, { products: [
    { product_id: "savings", name: "Savings", rate_bps: 650, notice_days: 0, available: true },
    { product_id: "notice-32", name: "32-day notice", rate_bps: 775, notice_days: 32, available: true } ] }));
  await page.route("**/api/v1/deposit-accounts", (r) => ok(r, { accounts: [{ account_id: "acc-n32", product_id: "notice-32", rate_bps: 775, notice_days: 32,
    balance_minor: "30000000", accrued_minor: "63699", under_notice_minor: "0", available_minor: "30000000", opened_at: "2026-10-01T09:00:00Z",
    interest_paid_minor: "191250", notices: [] }] }));
  await page.route("**/api/v1/cards", (r) => {
    if (r.request().method() === "POST") { cards = [card]; return ok(r, card, 201); }
    return ok(r, { cards });
  });
  await page.route("**/api/v1/cards/card-1/status", (r) => { cards = [{ ...card, status: r.request().postDataJSON().status }]; return ok(r, cards[0]); });
}

// A caption bar drawn over the real page.
async function caption(page, text) {
  await page.evaluate((t) => {
    let el = document.getElementById("__demo_cap");
    if (!el) {
      el = document.createElement("div"); el.id = "__demo_cap";
      el.style.cssText = "position:fixed;left:50%;bottom:28px;transform:translateX(-50%);z-index:99999;max-width:1000px;background:#111;color:#fff;" +
        "font:500 19px/1.4 system-ui,sans-serif;padding:12px 22px;border-radius:10px;box-shadow:0 6px 24px rgba(0,0,0,.3);pointer-events:none;text-align:center";
      document.body.appendChild(el);
    }
    el.textContent = t;
  }, text);
}

async function card_(page, title, sub, ms = 3500) {
  await page.setContent(`<body style="margin:0;height:100vh;display:grid;place-items:center;background:#ece8e0;font-family:system-ui,sans-serif;color:#111">
    <div style="text-align:center;max-width:900px"><div style="font:700 64px/1 system-ui;letter-spacing:.08em">FORGE</div>
    <div style="font:600 34px/1.3 system-ui;margin-top:28px">${title}</div><div style="font:400 20px/1.5 system-ui;margin-top:16px;color:#444">${sub}</div></div></body>`);
  await page.waitForTimeout(ms);
}

const pause = (p, ms = 1800) => p.waitForTimeout(ms);
const type = (loc, text) => loc.pressSequentially(text, { delay: 45 });

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: W, height: H }, recordVideo: { dir: out, size: { width: W, height: H } } });
const page = await ctx.newPage();
await mocks(page);

await card_(page, "Forge Mutual: the customer workspace", "A bank for technology companies and their founders.<br>Demo on sample data, with every outside party simulated. Not a licensed bank.", 5000);

await page.goto(BASE + "/sign-in");
await caption(page, "Sign in (development sign-in shown; production uses OIDC with MFA)");
await pause(page);
await type(page.getByLabel("Party ID"), "11111111-1111-4111-8111-111111111111");
await type(page.getByLabel("Your name"), "Thandi");
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL("**/accounts");
await caption(page, "Rand and US-dollar accounts, reconciled to the sponsor bank's ledger");
await pause(page, 4000);

await page.goto(BASE + "/fx");
await caption(page, "Foreign exchange: a firm quote with the spread disclosed in rand");
await pause(page);
await type(page.getByLabel("Amount (ZAR)"), "50000");
await page.getByRole("button", { name: "Get a quote" }).click();
await pause(page, 3500);
await caption(page, "The customer must accept the disclosure before the conversion can be booked");
await page.getByRole("checkbox").check();
await pause(page);
await page.getByRole("button", { name: "Book this conversion" }).click();
await pause(page, 3000);

await page.goto(BASE + "/payments/new");
await caption(page, "Payments: sent once, with an idempotency key, held if reconciliation has a break");
await pause(page, 1200);
await type(page.getByLabel("Amount (R)"), "18500");
await type(page.getByLabel("Beneficiary name"), "Lerato Studio (Pty) Ltd");
await type(page.getByLabel("Account number"), "1234567890");
await type(page.getByLabel("Bank code"), "250655");
await type(page.getByLabel("Reference"), "INV-0042");
await pause(page, 800);
await page.getByRole("button", { name: "Send payment" }).click();
await pause(page, 3000);

await page.goto(BASE + "/savings");
await caption(page, "Savings and notice deposits, with interest accrued daily");
await pause(page, 4500);

await page.goto(BASE + "/cards");
await caption(page, "Cards (switched off until the licence allows): issue, limits, freeze");
await pause(page, 1500);
const form = page.getByRole("form", { name: "Get a card" });
await type(form.getByLabel("Name"), "Cloud and tools");
await form.getByRole("button", { name: "Get card" }).click();
await pause(page, 2000);
await page.getByRole("listitem", { name: "Cloud and tools" }).getByRole("button", { name: "Freeze" }).click();
await caption(page, "Freeze a card in one click");
await pause(page, 3000);

await page.goto(BASE + "/apply");
await caption(page, "Applying: business and personal onboarding, with identity checks and screening");
await pause(page, 4000);

await card_(page, "Owned by its members", "Surplus builds the capital the law requires, then returns to members as lower fees and better rates.<br>Next: sponsor bank, then registration as a mutual bank.", 5000);

const video = page.video();
await ctx.close();
await browser.close();
console.log(await video.path());
