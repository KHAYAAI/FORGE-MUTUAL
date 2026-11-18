// Package finfake is a stateful stand-in for the Fineract endpoints the bridge uses. It is for tests. It can be told to
// accept a write and then fail, which is the case the poster's check-before-retry exists for.
package finfake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/fineract"
)

type entry struct {
	GL          int64
	Ref         string
	Txn         string
	Debit       bool
	AmountMajor string
	Currency    string
}

// Fake is the server state.
type Fake struct {
	mu        sync.Mutex
	nextID    int64
	gl        map[string]int64 // glCode to id
	entries   []entry
	postCount map[string]int // POSTs per reference
	// AcceptThenFail makes the next journal POST succeed on the server and answer 500.
	AcceptThenFail bool
	// RefuseNext makes the next journal POST answer 400 without writing.
	RefuseNext bool
}

// New returns an empty fake.
func New() *Fake {
	return &Fake{gl: map[string]int64{}, postCount: map[string]int{}, nextID: 1}
}

// Entries returns how many journal lines the fake holds for a reference.
func (f *Fake) Journals(ref string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.postCount[ref]
}

// Serve starts an httptest server and returns the fineract client for it.
func (f *Fake) Serve(t *testing.T) *fineract.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(srv.Close)
	return &fineract.Client{BaseURL: srv.URL + "/fineract-provider/api/v1", Tenant: "default", User: "u", Password: "p", HTTP: srv.Client()}
}

// Balance returns the fake's own balance for a GL account, in minor units, for assertions in tests.
func (f *Fake) Balance(glID int64, debitNormal bool) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	var total int64
	for _, e := range f.entries {
		if e.GL != glID {
			continue
		}
		amt := majorToMinor(e.AmountMajor)
		if e.Debit == debitNormal {
			total += amt
		} else {
			total -= amt
		}
	}
	return total
}

func majorToMinor(s string) int64 {
	var whole, frac int64
	var neg bool
	i := 0
	if i < len(s) && s[i] == '-' {
		neg = true
		i++
	}
	for ; i < len(s) && s[i] != '.'; i++ {
		whole = whole*10 + int64(s[i]-'0')
	}
	digits := 0
	if i < len(s) && s[i] == '.' {
		i++
		for ; i < len(s) && digits < 2; i++ {
			frac = frac*10 + int64(s[i]-'0')
			digits++
		}
	}
	for ; digits < 2; digits++ {
		frac *= 10
	}
	v := whole*100 + frac
	if neg {
		return -v
	}
	return v
}

func (f *Fake) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/fineract-provider/api/v1/glaccounts":
		out := []map[string]any{}
		for code, id := range f.gl {
			out = append(out, map[string]any{"id": id, "glCode": code, "name": code})
		}
		_ = json.NewEncoder(w).Encode(out)
	case r.Method == http.MethodPost && r.URL.Path == "/fineract-provider/api/v1/glaccounts":
		var body struct {
			GLCode string `json:"glCode"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := f.nextID
		f.nextID++
		f.gl[body.GLCode] = id
		_ = json.NewEncoder(w).Encode(map[string]any{"resourceId": id})
	case r.Method == http.MethodPost && r.URL.Path == "/fineract-provider/api/v1/journalentries":
		f.postJournal(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/fineract-provider/api/v1/journalentries":
		f.listEntries(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *Fake) postJournal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ref      string `json:"referenceNumber"`
		Currency string `json:"currencyCode"`
		Debits   []struct {
			GL     int64  `json:"glAccountId"`
			Amount string `json:"amount"`
		} `json:"debits"`
		Credits []struct {
			GL     int64  `json:"glAccountId"`
			Amount string `json:"amount"`
		} `json:"credits"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if f.RefuseNext {
		f.RefuseNext = false
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"developerMessage":"refused"}]}`))
		return
	}
	txn := fmt.Sprintf("txn-%d", len(f.entries)+1)
	for _, d := range body.Debits {
		f.entries = append(f.entries, entry{GL: d.GL, Ref: body.Ref, Txn: txn, Debit: true, AmountMajor: d.Amount, Currency: body.Currency})
	}
	for _, c := range body.Credits {
		f.entries = append(f.entries, entry{GL: c.GL, Ref: body.Ref, Txn: txn, Debit: false, AmountMajor: c.Amount, Currency: body.Currency})
	}
	f.postCount[body.Ref]++
	if f.AcceptThenFail {
		f.AcceptThenFail = false
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"developerMessage":"gateway timeout after write"}]}`))
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"officeId": 1, "transactionId": txn})
}

func (f *Fake) listEntries(w http.ResponseWriter, r *http.Request) {
	gl, _ := strconv.ParseInt(r.URL.Query().Get("glAccountId"), 10, 64)
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	var matched []map[string]any
	for _, e := range f.entries {
		if e.GL != gl {
			continue
		}
		code := "journalEntryType.credit"
		if e.Debit {
			code = "journalEntryType.debit"
		}
		matched = append(matched, map[string]any{
			"referenceNumber": e.Ref, "transactionId": e.Txn, "amount": json.Number(e.AmountMajor),
			"entryType": map[string]any{"code": code},
		})
	}
	total := len(matched)
	end := offset + 200
	if end > total {
		end = total
	}
	page := []map[string]any{}
	if offset < total {
		page = matched[offset:end]
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"totalFilteredRecords": total, "pageItems": page})
}
