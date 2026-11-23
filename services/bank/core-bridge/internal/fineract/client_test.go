package fineract

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A stub that records the last request and answers from a function.
func stub(t *testing.T, answer func(w http.ResponseWriter, r *http.Request, body []byte)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		answer(w, r, b)
	}))
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, Tenant: "default", User: "mifos", Password: "pw", HTTP: srv.Client()}
}

// The journal must carry basic auth, the tenant header, the date format Fineract expects, and amounts as decimal strings.
func TestPostJournalSendsTheShapeFineractExpects(t *testing.T) {
	var got map[string]any
	c := stub(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		if u, p, ok := r.BasicAuth(); !ok || u != "mifos" || p != "pw" {
			t.Errorf("basic auth missing or wrong")
		}
		if r.Header.Get("Fineract-Platform-TenantId") != "default" {
			t.Errorf("tenant header = %q", r.Header.Get("Fineract-Platform-TenantId"))
		}
		_ = json.Unmarshal(body, &got)
		_, _ = w.Write([]byte(`{"officeId":1,"transactionId":"tx-1"}`))
	})
	id, err := c.PostJournal(context.Background(), Journal{
		OfficeID: 1, Currency: "ZAR", Date: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Reference: "ref-1",
		Debits: []Line{{GLAccountID: 2, AmountMinor: 15000}}, Credits: []Line{{GLAccountID: 1, AmountMinor: 15000}},
	})
	if err != nil || id != "tx-1" {
		t.Fatalf("post: %q, %v", id, err)
	}
	if got["transactionDate"] != "01 November 2026" || got["dateFormat"] != "dd MMMM yyyy" || got["referenceNumber"] != "ref-1" {
		t.Fatalf("body = %v", got)
	}
	debits := got["debits"].([]any)
	if debits[0].(map[string]any)["amount"] != "150.00" {
		t.Fatalf("amount must be a decimal string, got %v", debits[0])
	}
}

func TestMinorToMajor(t *testing.T) {
	cases := map[int64]string{0: "0.00", 5: "0.05", 15000: "150.00", 123456: "1234.56", -250: "-2.50"}
	for in, want := range cases {
		if got := MinorToMajor(in); got != want {
			t.Fatalf("MinorToMajor(%d) = %q, want %q", in, got, want)
		}
	}
}

// Finding a reference must page through every entry, not stop at the first page.
func TestFindByReferencePagesThroughAllEntries(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		off := r.URL.Query().Get("offset")
		switch off {
		case "0":
			_, _ = w.Write([]byte(`{"totalFilteredRecords":3,"pageItems":[{"referenceNumber":"a","transactionId":"x"},{"referenceNumber":"b","transactionId":"y"}]}`))
		case "2":
			_, _ = w.Write([]byte(`{"totalFilteredRecords":3,"pageItems":[{"referenceNumber":"target","transactionId":"found-here"}]}`))
		default:
			_, _ = w.Write([]byte(`{"totalFilteredRecords":3,"pageItems":[]}`))
		}
	})
	id, ok, err := c.FindByReference(context.Background(), 7, "target")
	if err != nil || !ok || id != "found-here" {
		t.Fatalf("find: %q %v %v", id, ok, err)
	}
	_, ok, _ = c.FindByReference(context.Background(), 7, "missing")
	if ok {
		t.Fatal("an unknown reference must not be found")
	}
}

// Balances are summed exactly. 0.10 plus 0.20 must be 30 minor units, which binary floats do not give.
func TestBalanceIsExactDecimalArithmetic(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"totalFilteredRecords":3,"pageItems":[
		  {"amount":0.10,"entryType":{"code":"journalEntryType.credit"}},
		  {"amount":0.20,"entryType":{"code":"journalEntryType.credit"}},
		  {"amount":0.05,"entryType":{"code":"journalEntryType.debit"}}]}`))
	})
	liability, err := c.Balance(context.Background(), 1, false)
	if err != nil || liability != 25 {
		t.Fatalf("liability balance = %d, %v; want 25 (credits 30 minus debit 5)", liability, err)
	}
	asset, _ := c.Balance(context.Background(), 1, true)
	if asset != -25 {
		t.Fatalf("asset balance = %d, want -25", asset)
	}
}

func TestNotFoundIsDistinct(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(http.StatusNotFound) })
	if _, err := c.GLAccounts(context.Background()); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("404 must map to ErrNotFound: %v", err)
	}
}
