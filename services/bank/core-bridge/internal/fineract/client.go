// Package fineract is the client for the Apache Fineract REST API, the core banking ledger for customer-facing
// accounts and the general ledger (ARCHITECTURE.md 8). The request and response shapes here were checked against a
// running Fineract instance (see the live test), not taken from documentation alone.
//
// Fineract does not reject a repeated reference number: a second journal with the same reference is accepted. The
// client therefore offers FindByReference, and the poster uses it before any retry whose outcome is unknown.
package fineract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client talks to one Fineract tenant.
type Client struct {
	BaseURL  string // for example http://fineract:8080/fineract-provider/api/v1
	Tenant   string
	User     string
	Password string
	HTTP     *http.Client
}

// ErrNotFound is returned for a GL account that does not exist.
var ErrNotFound = errors.New("fineract: not found")

// ErrUnreachable is returned when no response arrived. The outcome of a write is then unknown: Fineract may have
// accepted it.
var ErrUnreachable = errors.New("fineract: unreachable")

// APIError is a response with a failure status. A 4xx means the request was refused and nothing was written. A 5xx
// means the outcome is unknown, because the server may have written before it failed.
type APIError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("fineract: %s %s returned %d: %s", e.Method, e.Path, e.Status, e.Body)
}

// Definitive reports whether the write certainly did not happen.
func (e *APIError) Definitive() bool { return e.Status >= 400 && e.Status < 500 }

// Uncertain reports whether a write might have happened despite the error.
func Uncertain(err error) bool {
	if errors.Is(err, ErrUnreachable) {
		return true
	}
	var api *APIError
	return errors.As(err, &api) && !api.Definitive()
}

// Line is one debit or credit of a journal entry. Amount is in minor units; the client formats it for Fineract.
type Line struct {
	GLAccountID int64
	AmountMinor int64
}

// Journal is one double-entry journal.
type Journal struct {
	OfficeID  int64
	Currency  string
	Date      time.Time
	Reference string
	Comments  string
	Debits    []Line
	Credits   []Line
}

// PostJournal posts a journal and returns Fineract's transaction id.
func (c *Client) PostJournal(ctx context.Context, j Journal) (string, error) {
	if len(j.Debits) == 0 || len(j.Credits) == 0 || j.Reference == "" {
		return "", errors.New("fineract: a journal needs debits, credits and a reference")
	}
	body := map[string]any{
		"officeId":        j.OfficeID,
		"transactionDate": j.Date.UTC().Format("02 January 2006"),
		"currencyCode":    j.Currency,
		"referenceNumber": j.Reference,
		"comments":        j.Comments,
		"dateFormat":      "dd MMMM yyyy",
		"locale":          "en",
		"debits":          lines(j.Debits),
		"credits":         lines(j.Credits),
	}
	var out struct {
		TransactionID string `json:"transactionId"`
	}
	if err := c.do(ctx, http.MethodPost, "/journalentries", body, &out); err != nil {
		return "", err
	}
	if out.TransactionID == "" {
		return "", errors.New("fineract: the journal was accepted without a transaction id")
	}
	return out.TransactionID, nil
}

func lines(ls []Line) []map[string]any {
	out := make([]map[string]any, 0, len(ls))
	for _, l := range ls {
		out = append(out, map[string]any{"glAccountId": l.GLAccountID, "amount": MinorToMajor(l.AmountMinor)})
	}
	return out
}

// MinorToMajor formats minor units as a decimal string with two places, for example 15000 becomes "150.00".
func MinorToMajor(minor int64) string {
	neg := minor < 0
	if neg {
		minor = -minor
	}
	s := fmt.Sprintf("%d.%02d", minor/100, minor%100)
	if neg {
		return "-" + s
	}
	return s
}

// FindByReference reports whether a journal with this reference already touches the GL account. It scans the account's
// journal entries page by page. It is used only after an uncertain outcome, so the cost is paid rarely.
func (c *Client) FindByReference(ctx context.Context, glAccountID int64, reference string) (string, bool, error) {
	for offset := 0; ; {
		var page struct {
			TotalFilteredRecords int `json:"totalFilteredRecords"`
			PageItems            []struct {
				ReferenceNumber string `json:"referenceNumber"`
				TransactionID   string `json:"transactionId"`
			} `json:"pageItems"`
		}
		q := url.Values{"glAccountId": {strconv.FormatInt(glAccountID, 10)}, "limit": {"200"}, "offset": {strconv.Itoa(offset)}}
		if err := c.do(ctx, http.MethodGet, "/journalentries?"+q.Encode(), nil, &page); err != nil {
			return "", false, err
		}
		for _, it := range page.PageItems {
			if it.ReferenceNumber == reference {
				return it.TransactionID, true, nil
			}
		}
		offset += len(page.PageItems)
		if len(page.PageItems) == 0 || offset >= page.TotalFilteredRecords {
			return "", false, nil
		}
	}
}

// GLAccount is one general ledger account.
type GLAccount struct {
	ID   int64
	Code string
	Name string
}

// GLAccounts lists every general ledger account.
func (c *Client) GLAccounts(ctx context.Context) ([]GLAccount, error) {
	var raw []struct {
		ID     int64  `json:"id"`
		GLCode string `json:"glCode"`
		Name   string `json:"name"`
	}
	if err := c.do(ctx, http.MethodGet, "/glaccounts", nil, &raw); err != nil {
		return nil, err
	}
	out := make([]GLAccount, 0, len(raw))
	for _, r := range raw {
		out = append(out, GLAccount{ID: r.ID, Code: r.GLCode, Name: r.Name})
	}
	return out, nil
}

// CreateGLAccount creates a detail account. kind is Fineract's account type: 1 asset, 2 liability, 3 equity,
// 4 income, 5 expense.
func (c *Client) CreateGLAccount(ctx context.Context, code, name string, kind int) (int64, error) {
	var out struct {
		ResourceID int64 `json:"resourceId"`
	}
	body := map[string]any{"name": name, "glCode": code, "manualEntriesAllowed": true, "type": kind, "usage": 1,
		"description": name, "parentId": nil}
	if err := c.do(ctx, http.MethodPost, "/glaccounts", body, &out); err != nil {
		return 0, err
	}
	return out.ResourceID, nil
}

// Balance sums the journal entries of a GL account, in minor units. For an asset the balance is debits minus credits;
// for a liability, income or equity it is credits minus debits. Amounts are parsed as decimal strings, never as floats.
func (c *Client) Balance(ctx context.Context, glAccountID int64, normalDebit bool) (int64, error) {
	total := new(big.Rat)
	for offset := 0; ; {
		var page struct {
			TotalFilteredRecords int `json:"totalFilteredRecords"`
			PageItems            []struct {
				Amount    json.Number `json:"amount"`
				EntryType struct {
					Code string `json:"code"`
				} `json:"entryType"`
			} `json:"pageItems"`
		}
		q := url.Values{"glAccountId": {strconv.FormatInt(glAccountID, 10)}, "limit": {"200"}, "offset": {strconv.Itoa(offset)}}
		if err := c.do(ctx, http.MethodGet, "/journalentries?"+q.Encode(), nil, &page); err != nil {
			return 0, err
		}
		for _, it := range page.PageItems {
			amt, ok := new(big.Rat).SetString(it.Amount.String())
			if !ok {
				return 0, fmt.Errorf("fineract: amount %q is not a decimal", it.Amount)
			}
			isDebit := strings.HasSuffix(it.EntryType.Code, ".debit")
			if isDebit == normalDebit {
				total.Add(total, amt)
			} else {
				total.Sub(total, amt)
			}
		}
		offset += len(page.PageItems)
		if len(page.PageItems) == 0 || offset >= page.TotalFilteredRecords {
			break
		}
	}
	minor := new(big.Rat).Mul(total, big.NewRat(100, 1))
	if !minor.IsInt() {
		return 0, errors.New("fineract: balance has more than two decimal places")
	}
	return minor.Num().Int64(), nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.User, c.Password)
	req.Header.Set("Fineract-Platform-TenantId", c.Tenant)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode >= 300 {
		return &APIError{Method: method, Path: path, Status: resp.StatusCode, Body: truncate(raw)}
	}
	if out == nil {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(out)
}

func truncate(b []byte) string {
	if len(b) > 300 {
		return string(b[:300]) + "..."
	}
	return string(b)
}
