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
