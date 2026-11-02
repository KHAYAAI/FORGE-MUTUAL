// Seeds the local stack through its real APIs, for the journey tests that run against it. Nothing here is mocked:
// each call goes to the service that owns the data, the same way the apps and the provider do.
//
// The provider's webhook is signed with the stack's KYC secret (deploy/compose/.env), exactly as a real provider
// would sign it. Every helper creates its own data, so journeys do not depend on each other or on earlier runs.
const { createHmac, randomBytes } = require("node:crypto");
const { readFileSync } = require("node:fs");
const { join } = require("node:path");

// CommonJS, so that the Playwright runner (which loads the journeys as CommonJS) and plain Node both load it.
const here = __dirname;
const ONBOARDING = "http://127.0.0.1:8083";
const BACKOFFICE = "http://127.0.0.1:8500";
const WORKSPACE = "http://127.0.0.1:8300";

const ONBOARDING_SERVICE = "forge.onboarding.v1.OnboardingService";

function stackSecret() {
  const env = readFileSync(join(here, "..", ".env"), "utf8");
  const line = env.split("\n").find((l) => l.startsWith("KYC_WEBHOOK_SECRET="));
  if (!line) throw new Error("KYC_WEBHOOK_SECRET is not in deploy/compose/.env: run deploy/compose/up.sh first");
  return line.slice("KYC_WEBHOOK_SECRET=".length);
}

async function connect(method, caller, body) {
  const res = await fetch(`${ONBOARDING}/${ONBOARDING_SERVICE}/${method}`, {
    method: "POST",
    headers: { "content-type": "application/json", "connect-protocol-version": "1", "x-forge-caller": caller },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`${method}: ${res.status} ${text}`);
  return JSON.parse(text);
}

async function until(label, check, timeoutMs = 60_000) {
  const deadline = Date.now() + timeoutMs;
  let last;
  while (Date.now() < deadline) {
    last = await check();
    if (last.done) return last.value;
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error(`timed out waiting for ${label}; last seen: ${JSON.stringify(last?.seen ?? last)}`);
}

// A 13-digit South African identity number with a valid Luhn check digit, unique per call. The party service refuses
// an identity it already holds, so every journey needs its own.
function nationalId() {
  const digits = Array.from({ length: 12 }, (_, i) => (i === 0 ? 7 + Math.floor(Math.random() * 2) : Math.floor(Math.random() * 10)));
  // Double every second digit counting from the right, starting with the rightmost payload digit; the check digit
  // makes the total a multiple of ten.
  let sum = 0;
  digits.forEach((d, i) => {
    const fromRight = digits.length - i; // the check digit is position 0
    let v = fromRight % 2 === 1 ? d * 2 : d;
    if (v > 9) v -= 9;
    sum += v;
  });
  return digits.join("") + String((10 - (sum % 10)) % 10);
}

/** Starts a business onboarding for a company with one director, and waits until its verifications have started. */
async function startCompany(label) {
  const digits = String(Math.floor(100000 + Math.random() * 899999));
  const { onboardingId } = await connect("StartBusinessOnboarding", "svc-backoffice-api", {
    idempotencyKey: `journey-${label}-${randomBytes(8).toString("hex")}`,
    legalName: `Journey ${label} (Pty) Ltd`,
    registrationNumber: `2026/${digits}/07`,
    country: "ZA",
    sectorCode: "software_saas",
    directors: [{ legalName: `Director ${label}`, nationalId: nationalId(), ownershipBps: 6000, isUbo: true }],
    actor: "journey",
  });
  const status = await until("verifications to start", async () => {
    const { status } = await connect("GetOnboarding", "svc-backoffice-api", { onboardingId });
    const started = (status.steps ?? []).includes("verifications_started");
    return { done: started && status.companyPartyId && status.personPartyIds?.length, value: status, seen: status.steps };
  });
  return { onboardingId, companyPartyId: status.companyPartyId, personPartyIds: status.personPartyIds };
}

/** Delivers the provider's verdict for one subject, signed as the provider signs it. */
async function verdict(onboardingId, subjectPartyId, value) {
  const body = JSON.stringify({
    session_id: `sess-${randomBytes(6).toString("hex")}`,
    onboarding_id: onboardingId,
    subject_party_id: subjectPartyId,
    verdict: value,
  });
  const signature = createHmac("sha256", stackSecret()).update(body).digest("hex");
  const res = await fetch(`${ONBOARDING}/webhooks/kyc`, {
    method: "POST",
    headers: { "content-type": "application/json", "x-forge-signature": signature },
    body,
  });
  if (res.status !== 202) throw new Error(`webhook ${value}: ${res.status} ${await res.text()}`);
}

async function onboardingState(onboardingId) {
  const { status } = await connect("GetOnboarding", "svc-backoffice-api", { onboardingId });
  return status;
}

function waitForState(onboardingId, state, timeoutMs = 60_000) {
  return until(`onboarding ${state}`, async () => {
    const status = await onboardingState(onboardingId);
    return { done: status.state === state, value: status, seen: status.state };
  }, timeoutMs);
}

const staff = (name, roles) => ({ "x-forge-staff": name, "x-forge-roles": roles.join(",") });

/** The open review for one subject, from the compliance queue. */
function waitForReview(onboardingId, subjectPartyId, timeoutMs = 60_000) {
  return until("the compliance review", async () => {
    const res = await fetch(`${BACKOFFICE}/v1/kyc/reviews`, { headers: staff("Naledi", ["compliance"]) });
    const { reviews = [] } = await res.json();
    const found = reviews.find((r) => r.onboarding_id === onboardingId && r.subject_party_id === subjectPartyId);
    return { done: found, value: found, seen: reviews.length };
  }, timeoutMs);
}

/** Records a decision through the back-office API as a compliance person. */
async function decideReview(reviewId, decision, note) {
  const res = await fetch(`${BACKOFFICE}/v1/kyc/reviews/${reviewId}/decision`, {
    method: "POST",
    headers: { "content-type": "application/json", ...staff("Naledi", ["compliance"]) },
    body: JSON.stringify({ decision, note }),
  });
  if (!res.ok) throw new Error(`decision: ${res.status} ${await res.text()}`);
  return res.json();
}

/** Waits until the ledger-backed read model shows an account for the party (the reconciler runs every ten seconds). */
function waitForAccounts(partyId, timeoutMs = 90_000) {
  return until("the account to be visible", async () => {
    const res = await fetch(`${WORKSPACE}/v1/accounts`, { headers: { "x-forge-party": partyId, "x-forge-actor": "journey" } });
    const { accounts = [] } = await res.json();
    return { done: accounts.length > 0, value: accounts, seen: accounts.length };
  }, timeoutMs);
}

module.exports = {
  ONBOARDING, BACKOFFICE, WORKSPACE,
  stackSecret, nationalId, startCompany, verdict, onboardingState, waitForState, waitForReview, decideReview, waitForAccounts,
};
