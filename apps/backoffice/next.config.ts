import type { NextConfig } from "next";

// The browser never talks to workspace-api directly. Requests go to /api/v1, which the route handler forwards with the
// session's identity headers, so the page holds no credentials of its own.
const config: NextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
};

export default config;
