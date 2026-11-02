"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { type FormEvent, useEffect, useState } from "react";
import Notice from "@/components/Notice";
import { api, errorText } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatMinor, parseToMinor } from "@/lib/money";

type Account = components["schemas"]["Account"];

export default function NewPaymentPage() {
  const router = useRouter();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountId, setAccountId] = useState("");
  const [amount, setAmount] = useState("");
  const [beneficiary, setBeneficiary] = useState("");
  const [accountNumber, setAccountNumber] = useState("");
  const [bankCode, setBankCode] = useState("");
  const [reference, setReference] = useState("");
  // One key per form. A double click or a retry sends the same key, so the payment is made once.
  const [idemKey, setIdemKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [sentId, setSentId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setIdemKey(crypto.randomUUID());
    api.GET("/v1/accounts").then(({ data, response }) => {
      if (response.status === 401) return router.push("/sign-in");
      const zar = (data?.accounts ?? []).filter((a) => a.asset === "ZAR");
      setAccounts(zar);
      if (zar[0]) setAccountId(zar[0].account_id);
    });
  }, [router]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    const minor = parseToMinor(amount);
    if (!minor) return setError("Enter an amount greater than zero, with at most two decimal places.");
    if (!idemKey) return;
    setBusy(true);
    const { data, error, response } = await api.POST("/v1/payments", {
      params: { header: { "Idempotency-Key": idemKey } },
      body: {
        account_id: accountId,
        amount_minor: minor,
        asset: "ZAR",
        beneficiary_name: beneficiary,
        account_number: accountNumber,
        bank_code: bankCode,
        reference,
      },
    });
    setBusy(false);
    if (response.status === 401) return router.push("/sign-in");
    if (error || !data) return setError(errorText(error, "The payment was not sent. Try again."));
    setSentId(data.payment_id);
    setIdemKey(null);
  }

  if (sentId) {
    return (
      <Notice tone="success">
        Payment accepted.{" "}
        <Link href={`/payments/${sentId}`} className="underline">
          See its status
        </Link>
        .
      </Notice>
    );
  }

  const selected = accounts.find((a) => a.account_id === accountId);

  return (
    <section className="max-w-xl space-y-6">
      <h1 className="text-2xl font-semibold">Send a payment</h1>
      {error ? <Notice tone="error">{error}</Notice> : null}
      <form onSubmit={submit} className="space-y-4" aria-label="Send a payment">
        <label className="block text-sm">
          <span className="font-medium">From</span>
          <select
            name="account"
            value={accountId}
            onChange={(e) => setAccountId(e.target.value)}
            required
            className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2 text-sm"
          >
            {accounts.map((a) => (
              <option key={a.account_id} value={a.account_id}>
                ZAR account, {formatMinor(a.available_minor, a.asset)}
              </option>
            ))}
          </select>
        </label>
        <label className="block text-sm">
          <span className="font-medium">Amount (R)</span>
          <input
            name="amount"
            inputMode="decimal"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            required
            placeholder="0.00"
            className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2 text-sm tabular-nums"
          />
        </label>
        <label className="block text-sm">
          <span className="font-medium">Beneficiary name</span>
          <input
            name="beneficiary"
            value={beneficiary}
            onChange={(e) => setBeneficiary(e.target.value)}
            required
            maxLength={140}
            className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2 text-sm"
          />
        </label>
        <label className="block text-sm">
          <span className="font-medium">Account number</span>
          <input
            name="accountNumber"
            value={accountNumber}
            onChange={(e) => setAccountNumber(e.target.value)}
            required
            className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2 text-sm"
          />
        </label>
        <label className="block text-sm">
          <span className="font-medium">Bank code</span>
          <input
            name="bankCode"
            value={bankCode}
            onChange={(e) => setBankCode(e.target.value)}
            required
            className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2 text-sm"
          />
        </label>
        <label className="block text-sm">
          <span className="font-medium">Reference</span>
          <input
            name="reference"
            value={reference}
            onChange={(e) => setReference(e.target.value)}
            required
            maxLength={140}
            className="mt-1 w-full rounded-md border border-stone-300 bg-white px-3 py-2 text-sm"
          />
        </label>
        <button
          type="submit"
          disabled={busy || !selected}
          className="rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white hover:bg-stone-800 disabled:opacity-50"
        >
          {busy ? "Sending…" : "Send payment"}
        </button>
      </form>
    </section>
  );
}
