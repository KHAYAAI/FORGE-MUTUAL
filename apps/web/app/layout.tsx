import type { Metadata } from "next";
import { connection } from "next/server";
import Nav from "@/components/Nav";
import PageMotion from "@/components/PageMotion";
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
      <body className="min-h-screen bg-stone-50 text-stone-900 antialiased">
        <div className="lg:flex">
          <Nav />
          <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-8 sm:px-8 lg:py-10">
            <PageMotion>{children}</PageMotion>
          </main>
        </div>
      </body>
    </html>
  );
}
