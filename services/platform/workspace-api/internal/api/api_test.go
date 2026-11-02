package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1/consolidatedv1connect"
	fxv1 "github.com/khayaai/forge-mutual/pkg/api/forge/fx/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/fx/v1/fxv1connect"
	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	payoutsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payouts/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payouts/v1/payoutsv1connect"
)

// The tests use fake downstream clients. Each one records what it was asked, so a test can check exactly what the
// workspace forwarded. Only the methods a test uses are implemented; the others panic if called.

const (
	partyA = "11111111-1111-4111-8111-111111111111"
	partyB = "22222222-2222-4222-8222-222222222222"
)

type fakeAuth struct{}

func (fakeAuth) Session(r *http.Request) (Session, error) {
	p := r.Header.Get("X-Test-Party")
	if p == "" {
		return Session{}, errors.New("no session")
	}
	return Session{Party: p, Actor: r.Header.Get("X-Test-Actor")}, nil
}

type fakeConsolidated struct {
	consolidatedv1connect.ConsolidatedViewServiceClient
	accounts map[string][]*consolidatedv1.AccountView // by party
	asked    []string                                 // parties asked for balances
	activity *consolidatedv1.GetActivityRequest
}

func (f *fakeConsolidated) GetBalances(_ context.Context, req *connect.Request[consolidatedv1.GetBalancesRequest]) (*connect.Response[consolidatedv1.GetBalancesResponse], error) {
	f.asked = append(f.asked, req.Msg.PartyId)
	return connect.NewResponse(&consolidatedv1.GetBalancesResponse{Accounts: f.accounts[req.Msg.PartyId]}), nil
}

func (f *fakeConsolidated) GetActivity(_ context.Context, req *connect.Request[consolidatedv1.GetActivityRequest]) (*connect.Response[consolidatedv1.GetActivityResponse], error) {
	f.activity = req.Msg
	return connect.NewResponse(&consolidatedv1.GetActivityResponse{NextAfterSeq: 7}), nil
}

type fakePayments struct {
	paymentsv1connect.PaymentsServiceClient
	created []*paymentsv1.CreatePaymentRequest
	err     error
}

func (f *fakePayments) CreatePayment(_ context.Context, req *connect.Request[paymentsv1.CreatePaymentRequest]) (*connect.Response[paymentsv1.CreatePaymentResponse], error) {
	f.created = append(f.created, req.Msg)
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(&paymentsv1.CreatePaymentResponse{PaymentId: "pay-1"}), nil
}

type fakePayouts struct {
	payoutsv1connect.PayoutsServiceClient
	created  []*payoutsv1.CreateBatchRequest
	approved []*payoutsv1.ApproveBatchRequest
}

func (f *fakePayouts) CreateBatch(_ context.Context, req *connect.Request[payoutsv1.CreateBatchRequest]) (*connect.Response[payoutsv1.CreateBatchResponse], error) {
	f.created = append(f.created, req.Msg)
	return connect.NewResponse(&payoutsv1.CreateBatchResponse{Batch: &payoutsv1.Batch{BatchId: "batch-1", State: payoutsv1.BatchState_BATCH_STATE_DRAFT}}), nil
}

func (f *fakePayouts) ApproveBatch(_ context.Context, req *connect.Request[payoutsv1.ApproveBatchRequest]) (*connect.Response[payoutsv1.ApproveBatchResponse], error) {
	f.approved = append(f.approved, req.Msg)
	return connect.NewResponse(&payoutsv1.ApproveBatchResponse{Batch: &payoutsv1.Batch{BatchId: req.Msg.BatchId, State: payoutsv1.BatchState_BATCH_STATE_RUNNING}}), nil
}

type fakeFX struct {
	fxv1connect.FXServiceClient
	booked *fxv1.BookRequest
}

func (f *fakeFX) Book(_ context.Context, req *connect.Request[fxv1.BookRequest]) (*connect.Response[fxv1.BookResponse], error) {
	f.booked = req.Msg
	return connect.NewResponse(&fxv1.BookResponse{DealId: "deal-1", State: "BOOKED"}), nil
}

type rig struct {
	h        http.Handler
	cons     *fakeConsolidated
	payments *fakePayments
	payouts  *fakePayouts
	fx       *fakeFX
}

func newRig(t *testing.T) *rig {
	t.Helper()
	cons := &fakeConsolidated{accounts: map[string][]*consolidatedv1.AccountView{
		partyA: {{AccountId: "acc-zar", Asset: "ZAR", AvailableMinor: "5000", HoldingEntity: "BANK", Insured: true}},
		partyB: {{AccountId: "acc-b", Asset: "ZAR"}},
	}}
	pay := &fakePayments{}
	po := &fakePayouts{}
	fx := &fakeFX{}
	a := &API{Auth: fakeAuth{}, Services: Services{Consolidated: cons, Payments: pay, Payouts: po, FX: fx}}
	return &rig{h: a.Handler(), cons: cons, payments: pay, payouts: po, fx: fx}
}

func (r *rig) do(t *testing.T, method, path, party, actor, idem string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if party != "" {
		req.Header.Set("X-Test-Party", party)
	}
	if actor != "" {
		req.Header.Set("X-Test-Actor", actor)
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	rec := httptest.NewRecorder()
	r.h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %v: %s", err, rec.Body.String())
	}
	return out
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	e, _ := decodeBody(t, rec)["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func TestRoutesNeedASession(t *testing.T) {
	r := newRig(t)
	rec := r.do(t, "GET", "/v1/accounts", "", "", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", rec.Code)
	}
	if len(r.cons.asked) != 0 {
		t.Fatal("a request without a session must not reach a service")
	}
}

// The party is the session's. Accounts are read for that party, whatever the client might ask.
func TestAccountsAreTheSessionsOwn(t *testing.T) {
	r := newRig(t)
	rec := r.do(t, "GET", "/v1/accounts", partyA, "alice", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if len(r.cons.asked) != 1 || r.cons.asked[0] != partyA {
		t.Fatalf("balances asked for %v, want the session's party only", r.cons.asked)
	}
	accounts := decodeBody(t, rec)["accounts"].([]any)
	if len(accounts) != 1 || accounts[0].(map[string]any)["holding_entity"] != "BANK" {
		t.Fatalf("accounts = %v", accounts)
	}
}

// A party field in a body is refused. It is never used in place of the session's party.
func TestBodyPartyIsRefused(t *testing.T) {
	r := newRig(t)
	body := map[string]any{"account_id": "acc-zar", "amount_minor": "100", "asset": "ZAR", "party_id": partyB,
		"beneficiary_name": "x", "account_number": "1234567890", "bank_code": "250655", "reference": "r"}
	rec := r.do(t, "POST", "/v1/payments", partyA, "alice", "k1", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
	if len(r.payments.created) != 0 {
		t.Fatal("a refused body must not reach payments")
	}
}

func paymentBody(account, asset string) map[string]any {
	return map[string]any{"account_id": account, "amount_minor": "1000", "asset": asset,
		"beneficiary_name": "Thandi Dube", "account_number": "1234567890", "bank_code": "250655", "reference": "invoice 42"}
}

func TestPaymentNeedsAnIdempotencyKey(t *testing.T) {
	r := newRig(t)
	rec := r.do(t, "POST", "/v1/payments", partyA, "alice", "", paymentBody("acc-zar", "ZAR"))
	if rec.Code != http.StatusBadRequest || len(r.payments.created) != 0 {
		t.Fatalf("no key: status %d, payments %d", rec.Code, len(r.payments.created))
	}
}

// A payment from an account the party does not own is refused before payments is asked.
func TestPaymentFromAnotherPartysAccountIsRefused(t *testing.T) {
	r := newRig(t)
	rec := r.do(t, "POST", "/v1/payments", partyA, "alice", "k1", paymentBody("acc-b", "ZAR"))
	if rec.Code != http.StatusForbidden || errorCode(t, rec) != "permission_denied" {
		t.Fatalf("status %d code %s, want 403 permission_denied", rec.Code, errorCode(t, rec))
	}
	if len(r.payments.created) != 0 {
		t.Fatal("payments must not be asked for another party's account")
	}
}

func TestPaymentWrongAssetIsRefused(t *testing.T) {
	r := newRig(t)
	rec := r.do(t, "POST", "/v1/payments", partyA, "alice", "k1", paymentBody("acc-zar", "USD"))
	if rec.Code != http.StatusBadRequest || len(r.payments.created) != 0 {
		t.Fatalf("status %d, payments %d; want 400 and no payment", rec.Code, len(r.payments.created))
	}
}

// The payment carries the session's party and actor, and a key scoped to the party, so two parties using the same key
// do not collide downstream.
func TestPaymentIsScopedToTheSession(t *testing.T) {
	r := newRig(t)
	rec := r.do(t, "POST", "/v1/payments", partyA, "alice", "k1", paymentBody("acc-zar", "ZAR"))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	got := r.payments.created[0]
	if got.PartyId != partyA || got.Actor != "alice" || got.IdempotencyKey != partyA+":k1" {
		t.Fatalf("forwarded = party %s actor %s key %s", got.PartyId, got.Actor, got.IdempotencyKey)
	}
	if got.AmountMinor != "1000" || got.FromAccountId != "acc-zar" {
		t.Fatalf("forwarded fields = %+v", got)
	}
	if decodeBody(t, rec)["payment_id"] != "pay-1" {
		t.Fatalf("response = %s", rec.Body.String())
	}
}

// A downstream refusal reaches the client with its meaning: a replayed key with a different request is a conflict.
func TestDownstreamRefusalsKeepTheirMeaning(t *testing.T) {
	r := newRig(t)
	r.payments.err = connect.NewError(connect.CodeAlreadyExists, errors.New("idempotency key reused with a different request"))
	rec := r.do(t, "POST", "/v1/payments", partyA, "alice", "k1", paymentBody("acc-zar", "ZAR"))
	if rec.Code != http.StatusConflict || errorCode(t, rec) != "already_exists" {
		t.Fatalf("status %d code %s, want 409 already_exists", rec.Code, errorCode(t, rec))
	}
	r.payments.err = connect.NewError(connect.CodeUnavailable, errors.New("workflow service unavailable"))
	rec = r.do(t, "POST", "/v1/payments", partyA, "alice", "k2", paymentBody("acc-zar", "ZAR"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable: status %d, want 503", rec.Code)
	}
}

func TestActivityBoundsAreChecked(t *testing.T) {
	r := newRig(t)
	for _, q := range []string{"?limit=0", "?limit=501", "?limit=abc", "?after=-1"} {
		rec := r.do(t, "GET", "/v1/accounts/acc-zar/activity"+q, partyA, "alice", "", nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", q, rec.Code)
		}
	}
	rec := r.do(t, "GET", "/v1/accounts/acc-zar/activity?after=3&limit=20", partyA, "alice", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if r.cons.activity.PartyId != partyA || r.cons.activity.AccountId != "acc-zar" || r.cons.activity.AfterSeq != 3 || r.cons.activity.Limit != 20 {
		t.Fatalf("activity request = %+v", r.cons.activity)
	}
}

// A batch is created by the session's actor, and the checker's approval takes the actor from the session, not the body.
func TestBatchMakerAndCheckerComeFromTheSession(t *testing.T) {
	r := newRig(t)
	body := map[string]any{"from_account_id": "acc-zar", "asset": "ZAR", "items": []map[string]any{{
		"item_key": "k1", "amount_minor": "1000", "beneficiary_name": "Thandi", "account_number": "1234567890",
		"bank_code": "250655", "reference": "inv"}}}
	rec := r.do(t, "POST", "/v1/payout-batches", partyA, "maker", "b1", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d: %s", rec.Code, rec.Body.String())
	}
	if c := r.payouts.created[0]; c.Actor != "maker" || c.PartyId != partyA || c.IdempotencyKey != partyA+":b1" || len(c.Items) != 1 {
		t.Fatalf("created = %+v", c)
	}
}

func TestApproveForwardsTheSessionActor(t *testing.T) {
	r := newRig(t)
	rec := r.do(t, "POST", "/v1/payout-batches/batch-1/approve", partyA, "checker", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve: status %d: %s", rec.Code, rec.Body.String())
	}
	if len(r.payouts.approved) != 1 || r.payouts.approved[0].Actor != "checker" || r.payouts.approved[0].BatchId != "batch-1" {
		t.Fatalf("approved = %+v", r.payouts.approved)
	}
}

func TestFXBookUsesTheSessionParty(t *testing.T) {
	r := newRig(t)
	rec := r.do(t, "POST", "/v1/fx/deals", partyA, "alice", "", map[string]any{
		"quote_id": "q1", "sell_account_id": "acc-zar", "buy_account_id": "acc-usd"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if r.fx.booked.PartyId != partyA || r.fx.booked.QuoteId != "q1" {
		t.Fatalf("booked = %+v", r.fx.booked)
	}
}

func TestUnknownFieldsAreRefusedOnQuotes(t *testing.T) {
	r := newRig(t)
	rec := r.do(t, "POST", "/v1/fx/quotes", partyA, "alice", "", map[string]any{
		"sell_asset": "ZAR", "buy_asset": "USD", "sell_amount_minor": "100", "spread_bps": 0})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
}
