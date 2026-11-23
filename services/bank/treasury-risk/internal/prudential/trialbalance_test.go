package prudential

import "testing"

// A deposit, a loan, interest and a fee: the trial balance nets to zero per ledger, and the balance sheet ties: assets
// less liabilities equals what the internal accounts hold.
func TestTheTrialBalanceAndBalanceSheetTie(t *testing.T) {
	bs := []Balance{
		{"ZAR", 3003, 1_000_00, 0}, // the sponsor holds R1 000 for us
		{"ZAR", 1001, 0, 1_000_00}, // a customer deposited it
		{"ZAR", 1020, 500_00, 0},   // a loan of R500 paid out ...
		{"ZAR", 1001, 0, 500_00},   // ... into a customer's account
		{"ZAR", 1001, 10_00, 0},    // a fee taken ...
		{"ZAR", 2002, 0, 10_00},    // ... to fees income
		{"USD", 5001, 100_00, 0},   // the partner holds $100 ...
		{"USD", 1001, 0, 100_00},   // ... for a customer
	}
	lines, nets := TrialBalance(bs)
	if nets["ZAR"] != 0 || nets["USD"] != 0 {
		t.Fatalf("ledgers do not net to zero: %v", nets)
	}
	if len(lines) != 6 || lines[0].Asset != "USD" {
		t.Fatalf("lines = %+v", lines)
	}
	sheet, assets, liabilities, internal, err := BalanceSheet(lines, map[string]int64{"USD": 18_00000000})
	if err != nil {
		t.Fatal(err)
	}
	// Assets: R1 000 + $100 x 18 = R1 800 cash, R500 loans = R3 300. Liabilities: R1 490 + R1 800 = R3 290. Fees R10.
	if assets != 3_300_00 || liabilities != 3_290_00 || internal != 10_00 || assets-liabilities != internal {
		t.Fatalf("assets %d liabilities %d internal %d; sheet %+v", assets, liabilities, internal, sheet)
	}
	if sheet[0].Section != "ASSETS" || sheet[len(sheet)-1].Section != "INTERNAL" {
		t.Fatalf("order: %+v", sheet)
	}
}

// A ledger that does not net to zero is reported, never hidden.
func TestAnUnbalancedLedgerIsReported(t *testing.T) {
	_, nets := TrialBalance([]Balance{{"ZAR", 1001, 0, 5}})
	if nets["ZAR"] == 0 {
		t.Fatal("an unbalanced ledger netted to zero")
	}
}
