import { NextResponse, type NextRequest } from "next/server";

// A fresh nonce for every page, and a Content-Security-Policy that runs only scripts carrying it. Next.js reads the
// nonce from the policy on the request and puts it on every script it renders, its own included. An injected script
// has no nonce, so the browser refuses it: 'unsafe-inline' is gone from script-src. 'strict-dynamic' lets a nonced
// script load the code-split chunks it needs.
//
// Styles keep 'unsafe-inline': React sets style attributes, which a nonce cannot cover. An injected style cannot run
// code; an injected script could.
export function proxy(request: NextRequest) {
  const nonce = Buffer.from(crypto.getRandomValues(new Uint8Array(18))).toString("base64");
  const development = process.env.NODE_ENV !== "production";
  const csp = [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${development ? " 'unsafe-eval'" : ""}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob:",
    "font-src 'self' data:",
    `connect-src 'self'${development ? " ws:" : ""}`,
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
  ].join("; ");

  const headers = new Headers(request.headers);
  headers.set("x-nonce", nonce);
  headers.set("Content-Security-Policy", csp);
  const response = NextResponse.next({ request: { headers } });
  response.headers.set("Content-Security-Policy", csp);
  return response;
}

export const config = {
  // Pages only: API routes return JSON and static assets carry no script.
  matcher: [{ source: "/((?!api|_next/static|_next/image|favicon.ico).*)" }],
};
