import type { NextConfig } from "next";

// The browser never talks to the APIs directly. Requests go to /api/v1, where the route handler forwards the session's
// bearer token; the page holds no credentials of its own.

// The Content-Security-Policy is set per request in proxy.ts, because each page needs its own nonce.
const headers = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), payment=()" },
  { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
  { key: "Strict-Transport-Security", value: "max-age=31536000" },
];

const config: NextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  async headers() {
    return [{ source: "/:path*", headers }];
  },
};

export default config;
