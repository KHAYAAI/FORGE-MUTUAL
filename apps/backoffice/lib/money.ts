// Money is a string of integer minor units everywhere, from the API to the screen. Formatting uses BigInt, so no amount
// ever passes through a float.

const MINOR_PER_MAJOR = 100n;

/** Formats minor units as "R 1 234.56" (ZAR) or "US$ 1 234.56" (USD). Unknown input is shown as it arrived. */
export function formatMinor(minor: string, asset: string): string {
  if (!/^-?\d+$/.test(minor)) return minor;
  const n = BigInt(minor);
  const negative = n < 0n;
  const abs = negative ? -n : n;
  const major = (abs / MINOR_PER_MAJOR).toString().replace(/\B(?=(\d{3})+(?!\d))/g, " ");
  const cents = (abs % MINOR_PER_MAJOR).toString().padStart(2, "0");
  const symbol = asset === "USD" ? "US$" : asset === "ZAR" ? "R" : asset;
  return `${negative ? "-" : ""}${symbol} ${major}.${cents}`;
}

/** Reads a typed amount such as "12.5" or "1 000.00" into minor units. Returns null when it is not a positive amount. */
export function parseToMinor(input: string): string | null {
  const cleaned = input.replace(/\s/g, "");
  if (!/^\d+(\.\d{1,2})?$/.test(cleaned)) return null;
  const [whole, frac = ""] = cleaned.split(".");
  const minor = BigInt(whole) * MINOR_PER_MAJOR + BigInt(frac.padEnd(2, "0"));
  if (minor <= 0n) return null;
  return minor.toString();
}
