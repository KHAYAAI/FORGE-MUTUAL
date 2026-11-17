package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	depositsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/deposits/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/deposits/v1/depositsv1connect"
)

type fakeDeposits struct {
	depositsv1connect.DepositsServiceClient
	withdraw *depositsv1.WithdrawRequest
}

func (f *fakeDeposits) Withdraw(_ context.Context, r *connect.Request[depositsv1.WithdrawRequest]) (*connect.Response[depositsv1.WithdrawResponse], error) {
	f.withdraw = r.Msg
	return connect.NewResponse(&depositsv1.WithdrawResponse{State: "NOTICE_GIVEN", Notice: &depositsv1.Notice{NoticeId: "n1", DueOn: "2026-12-01"}}), nil
}

// A withdrawal needs an idempotency key, carries the session party and actor, and its key is passed on.
func TestWithdrawalsUseTheSessionAndAKey(t *testing.T) {
	f := &fakeDeposits{}
	h := (&API{Auth: fakeAuth{}, Services: Services{Deposits: f}}).Handler()
	body := `{"from_account_id":"sav","to_account_id":"cur","amount_minor":"1000"}`
	req := httptest.NewRequest("POST", "/v1/withdrawals", bytes.NewBufferString(body))
	req.Header.Set("X-Test-Party", "party-session")
	req.Header.Set("X-Test-Actor", "Thandi")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("no idempotency key: %d", w.Code)
	}
	req = httptest.NewRequest("POST", "/v1/withdrawals", bytes.NewBufferString(body))
	req.Header.Set("X-Test-Party", "party-session")
	req.Header.Set("X-Test-Actor", "Thandi")
	req.Header.Set("Idempotency-Key", "k-1")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || f.withdraw.PartyId != "party-session" || f.withdraw.IdempotencyKey != "k-1" || f.withdraw.Actor != "Thandi" {
		t.Fatalf("withdraw: %d %s, sent %+v", w.Code, w.Body, f.withdraw)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"due_on":"2026-12-01"`)) {
		t.Fatalf("snake_case body expected: %s", w.Body)
	}
}

// Without a deposits service, the routes say savings are not offered.
func TestSavingsNotOfferedWithoutTheService(t *testing.T) {
	h := (&API{Auth: fakeAuth{}}).Handler()
	if w := asCustomer(t, h, "GET", "/v1/deposit-accounts", nil); w.Code != http.StatusNotFound {
		t.Fatalf("no deposits service: %d", w.Code)
	}
}
