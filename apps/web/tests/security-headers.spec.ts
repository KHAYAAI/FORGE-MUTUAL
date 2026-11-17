import { expect, test } from "@playwright/test";

// Every page and API response carries the security headers the production build sets (next.config.ts). The Content
// Security Policy must also forbid framing and plugins, which is what keeps the staff and customer screens from being
// embedded or clickjacked.
test("pages carry the security headers", async ({ request }) => {
  const res = await request.get("/sign-in");
  expect(res.status()).toBe(200);
  const h = res.headers();
  expect(h["x-content-type-options"]).toBe("nosniff");
  expect(h["x-frame-options"]).toBe("DENY");
  expect(h["referrer-policy"]).toBe("strict-origin-when-cross-origin");
  expect(h["cross-origin-opener-policy"]).toBe("same-origin");
  expect(h["strict-transport-security"]).toContain("max-age=");
  expect(h["x-powered-by"]).toBeUndefined();
  const csp = h["content-security-policy"] ?? "";
  expect(csp).toContain("frame-ancestors 'none'");
  expect(csp).toContain("object-src 'none'");
  expect(csp).toContain("default-src 'self'");
});

// Scripts run only with this request's nonce. Every script the page ships carries it, a second request gets a
// different one, and nothing in the policy lets an injected inline script run.
test("the policy allows only scripts that carry this request's nonce", async ({ request }) => {
  const nonceOf = (csp: string) => /script-src[^;]*'nonce-([A-Za-z0-9+/=]+)'/.exec(csp)?.[1];
  const first = await request.get("/sign-in");
  const csp = first.headers()["content-security-policy"] ?? "";
  const script = csp.split(";").find((d) => d.trim().startsWith("script-src")) ?? "";
  expect(script).not.toContain("'unsafe-inline'");
  expect(script).toContain("'strict-dynamic'");
  const nonce = nonceOf(csp);
  expect(nonce).toBeTruthy();

  const html = await first.text();
  const tags = html.match(/<script\b[^>]*>/g) ?? [];
  expect(tags.length).toBeGreaterThan(0);
  for (const tag of tags) expect(tag, tag).toContain(`nonce="${nonce}"`);

  const second = await request.get("/sign-in");
  expect(nonceOf(second.headers()["content-security-policy"] ?? "")).not.toBe(nonce);
});

// The page still works under that policy: it hydrates, and the browser reports no blocked script.
test("pages run under the strict policy with nothing blocked", async ({ page }) => {
  const blocked: string[] = [];
  page.on("console", (m) => {
    if (/Content Security Policy|Refused to (execute|load)/i.test(m.text())) blocked.push(m.text());
  });
  await page.goto("/sign-in");
  await expect(page.getByRole("button").first()).toBeVisible();
  await page.waitForLoadState("networkidle");
  expect(blocked).toEqual([]);
});

// What an attacker gets from a reflected or stored injection is markup in the page. A script tag added to the HTML the
// server sends, without the nonce, does not run; the page's own scripts still do.
test("a script injected into the page's HTML does not run", async ({ page }) => {
  await page.route("**/sign-in", async (route) => {
    const res = await route.fetch();
    const body = (await res.text()).replace("<body", "<script>window.__injected = true</script><body");
    await route.fulfill({ response: res, body, headers: res.headers() });
  });
  await page.goto("/sign-in");
  await expect(page.getByRole("button").first()).toBeVisible();
  expect(await page.evaluate(() => (window as unknown as { __injected?: boolean }).__injected === true)).toBe(false);
});
