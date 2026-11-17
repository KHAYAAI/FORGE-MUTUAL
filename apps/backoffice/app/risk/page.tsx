"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { formatMinor } from "@/lib/money";
import { useStaff } from "@/lib/useStaff";

type Scenario = { name: string; description: string; liquidity_left_minor: string; total_capital_bps: string; survives: boolean };
type Sheet = { section: string; label: string; rand_minor: string };
type Check = { asset: string; net_minor: string; balanced: boolean };
type TB = { as_of: string; balance_sheet: Sheet[]; checks: Check[]; total_assets_minor: string; total_liabilities_minor: string; internal_minor: string };
type Report = {
  as_of: string; capital_minor: string; total_rwa_minor: string; credit_rwa_minor: string; market_rwa_minor: string; operational_rwa_minor: string;
  total_capital_bps: string; min_total_capital_bps: string; leverage_bps: string; min_leverage_bps: string;
  hqla_minor: string; net_outflows_30d_minor: string; lcr_bps: string; min_lcr_bps: string;
  deposits_minor: string; loans_minor: string; largest_depositor_minor: string;
  breaches: string[]; stress: Scenario[]; limitations: string[];
};

const UNBOUNDED = 1n << 39n;
const pct = (bps: string) => (BigInt(bps) >= UNBOUNDED ? "no exposure" : `${(Number(bps) / 100).toFixed(2)}%`);
const NAMES: Record<string, string> = { TOTAL_CAPITAL_RATIO: "Total capital ratio", LEVERAGE_RATIO: "Leverage ratio", LIQUIDITY_COVERAGE: "Liquidity coverage" };

// Aggregate risk: capital, leverage, liquidity and stress, computed from the ledger by treasury-risk.
export default function RiskPage() {
  const router = useRouter();
  useStaff("/risk");
  const [r, setR] = useState<Report | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tb, setTb] = useState<TB | null>(null);

  useEffect(() => {
    fetch("/api/v1/risk/trial-balance", { cache: "no-store" }).then(async (res) => {
      if (res.ok) setTb((await res.json()) as TB);
    });
  }, []);

  useEffect(() => {
    fetch("/api/v1/risk/prudential", { cache: "no-store" }).then(async (res) => {
      if (res.status === 401) return router.push("/sign-in");
      if (!res.ok) return setError("The prudential report could not be loaded.");
      setR((await res.json()) as Report);
    });
  }, [router]);

  const Ratio = ({ label, value, min }: { label: string; value: string; min: string }) => {
    const breach = BigInt(value) < BigInt(min);
    return (
      <div className={`rounded-lg border p-4 ${breach ? "border-red-300 bg-red-50" : "border-stone-200 bg-white"}`}>
        <p className="text-sm text-stone-600">{label}</p>
        <p className="text-2xl font-semibold tabular-nums">{pct(value)}</p>
        <p className="text-sm text-stone-600">minimum {pct(min)}{breach ? " — in breach" : ""}</p>
      </div>
    );
  };

  return (
    <section aria-labelledby="risk-heading" className="space-y-6">
      <h1 id="risk-heading" className="text-2xl font-semibold">Prudential risk</h1>
      {error ? <Notice tone="error">{error}</Notice> : null}
      {r ? (
        <>
          <p className="text-sm text-stone-600">As of {r.as_of.replace("T", " ").slice(0, 16)} UTC. Weights, run-off rates and minimums are proposed until treasury sets them.</p>
          {r.breaches.length ? (
            <Notice tone="error">In breach: {r.breaches.map((b) => NAMES[b] ?? b).join(", ")}.</Notice>
          ) : (
            <Notice tone="success">Every ratio is above its minimum.</Notice>
          )}
          <div className="grid gap-4 sm:grid-cols-3">
            <Ratio label="Total capital ratio" value={r.total_capital_bps} min={r.min_total_capital_bps} />
            <Ratio label="Leverage ratio" value={r.leverage_bps} min={r.min_leverage_bps} />
            <Ratio label="Liquidity coverage (30 days)" value={r.lcr_bps} min={r.min_lcr_bps} />
          </div>
          <dl className="grid grid-cols-2 gap-2 rounded-lg border border-stone-200 bg-white p-4 text-sm sm:grid-cols-4">
            <dt className="text-stone-600">Capital</dt><dd>{formatMinor(r.capital_minor, "ZAR")}</dd>
            <dt className="text-stone-600">Risk-weighted assets</dt><dd>{formatMinor(r.total_rwa_minor, "ZAR")}</dd>
            <dt className="text-stone-600">Deposits</dt><dd>{formatMinor(r.deposits_minor, "ZAR")}</dd>
            <dt className="text-stone-600">Loans</dt><dd>{formatMinor(r.loans_minor, "ZAR")}</dd>
            <dt className="text-stone-600">Liquid assets</dt><dd>{formatMinor(r.hqla_minor, "ZAR")}</dd>
            <dt className="text-stone-600">Net outflows, 30 days</dt><dd>{formatMinor(r.net_outflows_30d_minor, "ZAR")}</dd>
            <dt className="text-stone-600">Largest depositor</dt><dd>{formatMinor(r.largest_depositor_minor, "ZAR")}</dd>
          </dl>
          <h2 className="text-lg font-semibold">Stress tests</h2>
          <table className="w-full text-left text-sm">
            <thead className="text-stone-600"><tr><th className="py-2">Scenario</th><th>Liquidity left</th><th>Capital ratio after</th><th>Result</th></tr></thead>
            <tbody>
              {r.stress.map((s) => (
                <tr key={s.name} className="border-t border-stone-200">
                  <td className="py-2">{s.description}</td>
                  <td className="tabular-nums">{formatMinor(s.liquidity_left_minor, "ZAR")}</td>
                  <td className="tabular-nums">{pct(s.total_capital_bps)}</td>
                  <td className={s.survives ? "text-emerald-800" : "font-semibold text-red-800"}>{s.survives ? "Survives" : "Fails"}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {tb ? (
            <div aria-labelledby="mgmt-heading" className="space-y-3">
              <h2 id="mgmt-heading" className="text-lg font-semibold">Management accounts</h2>
              {tb.checks.every((c) => c.balanced) ? (
                <Notice tone="success">Every ledger balances: debits equal credits in {tb.checks.map((c) => c.asset).join(", ")}.</Notice>
              ) : (
                <Notice tone="error">A ledger does not balance: {tb.checks.filter((c) => !c.balanced).map((c) => `${c.asset} by ${c.net_minor}`).join(", ")}.</Notice>
              )}
              <table className="w-full text-left text-sm">
                <tbody>
                  {tb.balance_sheet.map((l) => (
                    <tr key={l.section + l.label} className="border-t border-stone-200">
                      <td className="py-1 text-stone-600">{l.section === "ASSETS" ? "Asset" : l.section === "LIABILITIES" ? "Liability" : "Internal"}</td>
                      <td>{l.label}</td>
                      <td className="text-right tabular-nums">{formatMinor(l.rand_minor, "ZAR")}</td>
                    </tr>
                  ))}
                  <tr className="border-t-2 border-stone-400 font-semibold">
                    <td /><td>Assets less liabilities</td>
                    <td className="text-right tabular-nums">{formatMinor((BigInt(tb.total_assets_minor) - BigInt(tb.total_liabilities_minor)).toString(), "ZAR")}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          ) : null}
          <p className="text-sm">
            <a className="underline" href="/api/v1/risk/single-customer-view.csv">Download the single customer view</a>{" "}
            (deposit insurance: each depositor&apos;s covered and uncovered deposits; operations role).
          </p>
          <h2 className="text-lg font-semibold">What this report does not yet see</h2>
          <ul className="list-disc pl-5 text-sm text-stone-700">{r.limitations.map((l) => <li key={l}>{l}</li>)}</ul>
        </>
      ) : null}
    </section>
  );
}
