// Package chart maps the ledger's account codes to Fineract general ledger accounts. A GL account is per asset and per
// code ("1001-ZAR"), so a currency never mixes into another currency's balance. The map is built from the list of codes
// below and the assets in use, so it cannot drift from the ledger's codes (ARCHITECTURE.md Appendix B).
package chart

import "fmt"

// Kind is the Fineract account type: 1 asset, 2 liability, 3 equity, 4 income, 5 expense.
type Kind int

// Entry describes one ledger account code.
type Entry struct {
	Code  uint16
	Name  string
	Kind  Kind
	Debit bool // true when the balance is debit-normal (assets), false when credit-normal
}

// Codes are the ledger codes that core banking mirrors. Codes outside this list are not projected, and a transfer that
// touches one is recorded as FAILED with the reason, for an operator to resolve.
var Codes = []Entry{
	{Code: 1001, Name: "Customer deposits", Kind: 2, Debit: false},
	{Code: 2001, Name: "Operating", Kind: 2, Debit: false},
	{Code: 2002, Name: "Fees income", Kind: 4, Debit: false},
	{Code: 2005, Name: "Suspense", Kind: 2, Debit: false},
	{Code: 3001, Name: "Rail settlement EFT", Kind: 2, Debit: false},
	{Code: 3003, Name: "Sponsor pooled", Kind: 1, Debit: true},
	{Code: 5001, Name: "Partner bank mirror", Kind: 1, Debit: true},
}

// Assets are the currencies core banking mirrors.
var Assets = []string{"ZAR", "USD"}

// GLCode is the Fineract general ledger code for a ledger code in an asset.
func GLCode(code uint16, asset string) string { return fmt.Sprintf("%d-%s", code, asset) }

// Lookup returns the entry for a ledger code, if it is mirrored.
func Lookup(code uint16) (Entry, bool) {
	for _, e := range Codes {
		if e.Code == code {
			return e, true
		}
	}
	return Entry{}, false
}
