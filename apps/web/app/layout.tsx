import type { Metadata } from "next";
import { connection } from "next/server";
import Nav from "@/components/Nav";
import "./globals.css";

export const metadata: Metadata = {
  title: "Forge workspace",
  description: "Accounts, payments, FX and payouts for your business.",
};

// Every page is rendered per request, so it can carry that request's nonce (proxy.ts).
export default async function RootLayout({ children }: { children: React.ReactNode }) {
  await connection();
  return (
    <html lang="en-ZA">
      <body className="min-h-screen bg-white text-stone-900 antialiased">
        <Nav />
        <main className="mx-auto max-w-6xl px-4 py-10 sm:px-6">{children}</main>
      </body>
    </html>
  );
}
