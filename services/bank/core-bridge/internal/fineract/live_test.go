package fineract

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestLiveFineract runs against a real Fineract instance. It creates its own GL accounts, posts a journal, and checks
// that the balances and the reference lookup agree. It needs FINERACT_TEST_URL, FINERACT_TEST_USER and
// FINERACT_TEST_PASSWORD. Tenant is FINERACT_TEST_TENANT (default "default").
func TestLiveFineract(t *testing.T) {
	base := os.Getenv("FINERACT_TEST_URL")
	if base == "" {
		t.Skip("FINERACT_TEST_URL not set")
	}
	tenant := os.Getenv("FINERACT_TEST_TENANT")
	if tenant == "" {
		tenant = "default"
	}
	c := &Client{BaseURL: base, Tenant: tenant, User: os.Getenv("FINERACT_TEST_USER"), Password: os.Getenv("FINERACT_TEST_PASSWORD")}
	ctx := context.Background()
	suffix := time.Now().UnixNano() % 1_000_000_000
	asset, err := c.CreateGLAccount(ctx, fmt.Sprintf("8%09d", suffix), "core-bridge test asset", 1)
	if err != nil {
		t.Fatalf("create asset GL: %v", err)
	}
	liability, err := c.CreateGLAccount(ctx, fmt.Sprintf("7%09d", suffix), "core-bridge test liability", 2)
	if err != nil {
		t.Fatalf("create liability GL: %v", err)
	}
	before, err := c.Balance(ctx, liability, false)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	ref := fmt.Sprintf("live-%d", suffix)
	txn, err := c.PostJournal(ctx, Journal{OfficeID: 1, Currency: "ZAR", Date: time.Now(), Reference: ref, Comments: "live test",
		Debits: []Line{{GLAccountID: asset, AmountMinor: 15000}}, Credits: []Line{{GLAccountID: liability, AmountMinor: 15000}}})
	if err != nil || txn == "" {
		t.Fatalf("post: %q %v", txn, err)
	}
	found, ok, err := c.FindByReference(ctx, asset, ref)
	if err != nil || !ok || found != txn {
		t.Fatalf("find by reference: %q %v %v; want %q", found, ok, err, txn)
	}
	after, err := c.Balance(ctx, liability, false)
	if err != nil || after-before != 15000 {
		t.Fatalf("liability moved by %d (%v), want 15000", after-before, err)
	}
	if _, ok, _ := c.FindByReference(ctx, asset, ref+"-never"); ok {
		t.Fatal("an unposted reference must not be found")
	}
}
