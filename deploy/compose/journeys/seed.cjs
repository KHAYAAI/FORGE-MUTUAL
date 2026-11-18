// Seeds the local stack through its real APIs, for the journey tests that run against it. Nothing here is mocked:
// each call goes to the service that owns the data, the same way the apps and the provider do.
//
// The provider's webhook is signed with the stack's KYC secret (deploy/compose/.env), exactly as a real provider
// would sign it. Every helper creates its own data, so journeys do not depend on each other or on earlier runs.
const { createHmac, randomBytes } = require("node:crypto");
const { readFileSync } = require("node:fs");
const http = require("node:http");
const https = require("node:https");
const { join } = require("node:path");

// CommonJS, so that the Playwright runner (which loads the journeys as CommonJS) and plain Node both load it.
const here = __dirname;

/** A value from deploy/compose/.env, which up.sh writes. */
function stackEnv(name) {
  const env = readFileSync(join(here, "..", ".env"), "utf8");
  const line = env.split("\n").find((l) => l.startsWith(`${name}=`));
  return line ? line.slice(name.length + 1) : undefined;
}

// With `up.sh mtls` every service speaks TLS and requires a client certificate. The journeys then present their own,
// which names them svc-stack-journeys: an identity that exists only in the local stack, and is allowed exactly the
// onboarding calls the journeys make. Without it, the same name goes in the development caller header.
const TLS = stackEnv("STACK_TLS") === "1";
const JOURNEYS = "svc-stack-journeys";
const scheme = TLS ? "https" : "http";
const ONBOARDING = `${scheme}://127.0.0.1:8083`;
const BACKOFFICE = `${scheme}://127.0.0.1:8500`;
const WORKSPACE = `${scheme}://127.0.0.1:8300`;

const ONBOARDING_SERVICE = "forge.onboarding.v1.OnboardingService";

const pki = (file) => readFileSync(join(here, "..", ".pki", file));
const agent = TLS
  ? new https.Agent({ cert: pki(`${JOURNEYS}.crt`), key: pki(`${JOURNEYS}.key`), ca: pki("ca.crt"), keepAlive: true })
  : undefined;

/** One HTTP request. Like fetch, but able to present a client certificate. */
function send(url, { method = "GET", headers = {}, body } = {}) {
  return new Promise((resolve, reject) => {
    const req = (TLS ? https : http).request(url, { method, headers, agent }, (res) => {
      const chunks = [];
      res.on("data", (c) => chunks.push(c));
      res.on("end", () => {
        const text = Buffer.concat(chunks).toString("utf8");
        resolve({ status: res.statusCode, ok: res.statusCode >= 200 && res.statusCode < 300, text, json: () => JSON.parse(text) });
      });
    });
    req.on("error", reject);
    if (body !== undefined) req.write(body);
    req.end();
  });
}

function stackSecret() {
  const secret = stackEnv("KYC_WEBHOOK_SECRET");
  if (!secret) throw new Error("KYC_WEBHOOK_SECRET is not in deploy/compose/.env: run deploy/compose/up.sh first");
  return secret;
}

/** One onboarding call, returning the raw reply. */
function call(method, body) {
  return send(`${ONBOARDING}/${ONBOARDING_SERVICE}/${method}`, {
    method: "POST",
    headers: { "content-type": "application/json", "connect-protocol-version": "1", "x-forge-caller": JOURNEYS },
    body: JSON.stringify(body),
  });
}

async function connect(method, body) {
  const res = await call(method, body);
  if (!res.ok) throw new Error(`${method}: ${res.status} ${res.text}`);
  return res.json();
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
  // A real birth date (YYMMDD, a month and a day that exist), then six random digits.
  const pad = (n) => String(n).padStart(2, "0");
  const date = `${pad(70 + Math.floor(Math.random() * 30))}${pad(1 + Math.floor(Math.random() * 12))}${pad(1 + Math.floor(Math.random() * 28))}`;
  const digits = [...date, ...Array.from({ length: 6 }, () => String(Math.floor(Math.random() * 10)))].map(Number);
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
  const { onboardingId } = await connect("StartBusinessOnboarding", {
    idempotencyKey: `journey-${label}-${randomBytes(8).toString("hex")}`,
    legalName: `Journey ${label} (Pty) Ltd`,
    registrationNumber: `2026/${digits}/07`,
    country: "ZA",
    sectorCode: "software_saas",
    directors: [{ legalName: `Director ${label}`, nationalId: nationalId(), ownershipBps: 6000, isUbo: true }],
    actor: "journey",
  });
  const status = await until("verifications to start", async () => {
    const { status } = await connect("GetOnboarding", { onboardingId });
    const started = (status.steps ?? []).includes("verifications_started");
    return { done: started && status.companyPartyId && status.personPartyIds?.length, value: status, seen: status.steps };
  });
  return { onboardingId, companyPartyId: status.companyPartyId, personPartyIds: status.personPartyIds };
}

/**
 * Waits for the application a signed-in applicant made through the web app, and returns it once its verifications have
 * started. The subject is the applicant's sign-in subject (for the development sign-in, "dev-" and their name).
 */
async function findApplication(subject, kind, timeoutMs = 60_000) {
  return until(`${kind} application of ${subject}`, async () => {
    const res = await call("FindApplication", { applicantSubject: subject, kind });
    if (res.status === 404) return { done: false, seen: "not found" };
    if (!res.ok) throw new Error(`FindApplication: ${res.status} ${res.text}`);
    const { status } = res.json();
    const started = (status.steps ?? []).includes("verifications_started") && status.personPartyIds?.length;
    return { done: started, value: status, seen: status.steps };
  }, timeoutMs);
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
  const res = await send(`${ONBOARDING}/webhooks/kyc`, {
    method: "POST",
    headers: { "content-type": "application/json", "x-forge-signature": signature },
    body,
  });
  if (res.status !== 202) throw new Error(`webhook ${value}: ${res.status} ${res.text}`);
}

async function onboardingState(onboardingId) {
  const { status } = await connect("GetOnboarding", { onboardingId });
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
    const res = await send(`${BACKOFFICE}/v1/kyc/reviews`, { headers: staff("Naledi", ["compliance"]) });
    const { reviews = [] } = res.json();
    const found = reviews.find((r) => r.onboarding_id === onboardingId && r.subject_party_id === subjectPartyId);
    return { done: found, value: found, seen: reviews.length };
  }, timeoutMs);
}

/** Records a decision through the back-office API as a compliance person. */
async function decideReview(reviewId, decision, note) {
  const res = await send(`${BACKOFFICE}/v1/kyc/reviews/${reviewId}/decision`, {
    method: "POST",
    headers: { "content-type": "application/json", ...staff("Naledi", ["compliance"]) },
    body: JSON.stringify({ decision, note }),
  });
  if (!res.ok) throw new Error(`decision: ${res.status} ${res.text}`);
  return res.json();
}

/** Waits until the ledger-backed read model shows an account for the party (the reconciler runs every ten seconds). */
function waitForAccounts(partyId, timeoutMs = 90_000) {
  return until("the account to be visible", async () => {
    const res = await send(`${WORKSPACE}/v1/accounts`, { headers: { "x-forge-party": partyId, "x-forge-actor": "journey" } });
    const { accounts = [] } = res.json();
    return { done: accounts.length > 0, value: accounts, seen: accounts.length };
  }, timeoutMs);
}

module.exports = {
  ONBOARDING, BACKOFFICE, WORKSPACE, TLS,
  stackSecret, stackEnv, send, nationalId, startCompany, findApplication, verdict, onboardingState, waitForState, waitForReview, decideReview, waitForAccounts,
};
