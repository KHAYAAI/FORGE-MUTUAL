import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { reply } from "./helpers";

// The customer front door, with the workspace API mocked in the browser. The real thing runs in tests/stack.

const VALID_ID = "9001015009086";

async function signInAsApplicant(page: Page, next: string) {
  await page.goto(`/sign-in?next=${encodeURIComponent(next)}`);
  await page.getByLabel("Your name").fill("Naledi Mokoena");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL(`**${next}`);
}

async function accessible(page: Page, label: string) {
  const r = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  expect(
    r.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => `${v.id}: ${v.help}`),
    `${label} has serious or critical accessibility violations`,
  ).toEqual([]);
}

test("the front door offers both accounts, says the bank is not yet licensed, and is accessible", async ({ page }) => {
  await page.goto("/");
  await page.waitForURL("**/apply");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Banking for the people who build.");
  await expect(page.getByText("Forge Mutual is not yet a licensed bank")).toBeVisible();
  await expect(page.getByRole("link", { name: "Apply" })).toHaveCount(2);
  await accessible(page, "front door");
});

test("applying needs a sign-in, and the sign-in comes back to the form", async ({ page }) => {
  await page.goto("/apply/personal");
  await page.getByRole("link", { name: "Sign in to apply" }).click();
  await page.waitForURL("**/sign-in?next=%2Fapply%2Fpersonal");
  await page.getByLabel("Your name").fill("Naledi Mokoena");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/apply/personal");
  await expect(page.getByLabel("South African ID number")).toBeVisible();
});

test("a sign-in cannot be sent to another site", async ({ page }) => {
  await page.goto("/sign-in?next=//evil.example/steal");
  await page.getByLabel("Your name").fill("Naledi Mokoena");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/apply");
  expect(new URL(page.url()).host).toBe("127.0.0.1:3100");
});

test("a wrong ID number is caught before anything is sent", async ({ page }) => {
  let sent = false;
  await page.route("**/api/v1/applications/person", (route) => {
    sent = true;
    return reply(route, 202, {});
  });
  await signInAsApplicant(page, "/apply/personal");
  await page.getByLabel("Full legal name").fill("Naledi Mokoena");
  await page.getByLabel("South African ID number").fill("9001015009087");
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "Continue" }).click();
  await expect(page.getByText("That is not a valid South African ID number")).toBeVisible();
  expect(sent).toBe(false);
});

test("a personal application goes to verification, then the account opens", async ({ page }) => {
  let body: unknown = null;
  let state: "verify" | "approved" = "verify";
  await page.route("**/api/v1/applications/person", (route) => {
    if (route.request().method() === "POST") {
      body = route.request().postDataJSON();
      return reply(route, 202, { kind: "person", state: "in_progress", next: "wait", steps: ["prospects_created"] });
    }
    return state === "verify"
      ? reply(route, 200, {
          kind: "person", state: "in_progress", next: "verify",
          steps: ["prospects_created", "consents_recorded", "verifications_started"],
          verify: [{ kind: "person", url: "https://verify.example/session-1" }],
        })
      : reply(route, 200, { kind: "person", state: "approved", next: "", steps: ["prospects_created", "verifications_started", "kyc_approved", "screened", "accounts_created", "activated"], party_id: "33333333-3333-4333-8333-333333333333" });
  });
  await signInAsApplicant(page, "/apply/personal");
  await accessible(page, "personal application");
  await page.getByLabel("Full legal name").fill("Naledi Mokoena");
  await page.getByLabel("South African ID number").fill(VALID_ID);
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "Continue" }).click();
  await page.waitForURL("**/apply/status/person");
  expect(body).toEqual({ legal_name: "Naledi Mokoena", national_id: VALID_ID });

  await expect(page.getByRole("heading", { level: 1 })).toHaveText("One more step: verify your identity");
  const link = page.getByRole("link", { name: "Start verification" });
  await expect(link).toHaveAttribute("href", "https://verify.example/session-1");
  await expect(link).toHaveAttribute("rel", /noopener/);
  await accessible(page, "application status");

  state = "approved";
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Your account is open", { timeout: 10_000 });
  await expect(page.getByRole("link", { name: "Go to your account" })).toBeVisible();
});

test("a declined applicant is told so, and never why", async ({ page }) => {
  await page.route("**/api/v1/applications/person", (route) =>
    reply(route, 200, { kind: "person", state: "declined", next: "", steps: ["prospects_created"] }),
  );
  await signInAsApplicant(page, "/apply/status/person");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("We could not open your account");
  await expect(page.getByText(/screening|sanction|match/i)).toHaveCount(0);
});

test("a business application sends each director, with their share in basis points", async ({ page }) => {
  let body: { directors?: unknown[] } | null = null;
  await page.route("**/api/v1/applications/business", (route) => {
    if (route.request().method() === "POST") body = route.request().postDataJSON();
    return reply(route, 202, { kind: "business", state: "in_progress", next: "wait", steps: [] });
  });
  await signInAsApplicant(page, "/apply/business");
  await accessible(page, "business application");
  await page.getByLabel("Registered company name").fill("Forge Pilot (Pty) Ltd");
  await page.getByLabel("Registration number").fill("2026/123456/07");
  await page.getByRole("group", { name: "Director 1" }).getByLabel("Full legal name").fill("Thandi Dube");
  await page.getByRole("group", { name: "Director 1" }).getByLabel("South African ID number").fill("8001015009087");
  await page.getByRole("group", { name: "Director 1" }).getByLabel("Shareholding %").fill("60");
  await page.getByRole("group", { name: "Director 1" }).getByLabel("Beneficial owner").check();
  await page.getByRole("button", { name: "Add another director" }).click();
  await page.getByRole("group", { name: "Director 2" }).getByLabel("Full legal name").fill("Sipho Ndlovu");
  await page.getByRole("group", { name: "Director 2" }).getByLabel("South African ID number").fill(VALID_ID);
  await page.getByRole("checkbox", { name: /authorised/ }).check();
  await page.getByRole("button", { name: "Continue" }).click();
  await page.waitForURL("**/apply/status/business");
  expect(body).toEqual({
    legal_name: "Forge Pilot (Pty) Ltd",
    registration_number: "2026/123456/07",
    sector_code: "software_saas",
    directors: [
      { legal_name: "Thandi Dube", national_id: "8001015009087", ownership_bps: 6000, is_ubo: true },
      { legal_name: "Sipho Ndlovu", national_id: VALID_ID, ownership_bps: 0, is_ubo: false },
    ],
  });
});
