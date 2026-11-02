// Package api is the customer workspace's REST surface. It keeps no state. Each request is a session's view, forwarded to
// the service that owns the data. The party comes from the session, never from the request, and request bodies are decoded
// strictly, so a party field sent in a body is refused rather than used.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1/consolidatedv1connect"
	fxv1 "github.com/khayaai/forge-mutual/pkg/api/forge/fx/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/fx/v1/fxv1connect"
	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	payoutsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payouts/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payouts/v1/payoutsv1connect"
	"github.com/khayaai/forge-mutual/pkg/uuid"
)

// Session is who is calling, and for which party. It is set by an Authenticator, not by the request body.
type Session struct {
	Party string
	Actor string
}

// Authenticator turns a request into a session, or refuses it.
type Authenticator interface {
	Session(r *http.Request) (Session, error)
}

// Services are the downstream clients the workspace reads and writes through.
type Services struct {
	Consolidated consolidatedv1connect.ConsolidatedViewServiceClient
	Payments     paymentsv1connect.PaymentsServiceClient
	FX           fxv1connect.FXServiceClient
	Payouts      payoutsv1connect.PayoutsServiceClient
}

// API serves the workspace routes.
type API struct {
	Auth     Authenticator
	Services Services
}

// Handler returns the routes. Health checks are open; every other route needs a session.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /v1/accounts", a.session(a.listAccounts))
	mux.HandleFunc("GET /v1/accounts/{account}/activity", a.session(a.listActivity))
	mux.HandleFunc("POST /v1/payments", a.session(a.createPayment))
	mux.HandleFunc("GET /v1/payments/{payment}", a.session(a.getPayment))
	mux.HandleFunc("POST /v1/fx/quotes", a.session(a.quoteFX))
	mux.HandleFunc("POST /v1/fx/deals", a.session(a.bookFX))
	mux.HandleFunc("GET /v1/fx/deals/{deal}", a.session(a.getDeal))
	mux.HandleFunc("POST /v1/payout-batches", a.session(a.createBatch))
	mux.HandleFunc("GET /v1/payout-batches/{batch}", a.session(a.getBatch))
	mux.HandleFunc("POST /v1/payout-batches/{batch}/approve", a.session(a.approveBatch))
	return mux
}

type handler func(w http.ResponseWriter, r *http.Request, s Session)

func (a *API) session(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := a.Auth.Session(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errBody("unauthenticated", "sign in to continue"))
			return
		}
		h(w, r, s)
	}
}

// ---- accounts ----

type accountJSON struct {
	AccountID      string `json:"account_id"`
	Asset          string `json:"asset"`
	AvailableMinor string `json:"available_minor"`
	HoldingEntity  string `json:"holding_entity"`
	Insured        bool   `json:"insured"`
	ProductID      string `json:"product_id"`
	AsOf           string `json:"as_of"`
	Source         string `json:"source"`
}

func (a *API) listAccounts(w http.ResponseWriter, r *http.Request, s Session) {
	accounts, err := a.balances(r.Context(), s.Party)
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]accountJSON, 0, len(accounts))
	for _, x := range accounts {
		out = append(out, accountJSON{
			AccountID: x.AccountId, Asset: x.Asset, AvailableMinor: x.AvailableMinor, HoldingEntity: x.HoldingEntity,
			Insured: x.Insured, ProductID: x.ProductId, AsOf: x.AsOf, Source: x.Source,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": out})
}

func (a *API) balances(ctx context.Context, party string) ([]*consolidatedv1.AccountView, error) {
	res, err := a.Services.Consolidated.GetBalances(ctx, connect.NewRequest(&consolidatedv1.GetBalancesRequest{PartyId: party}))
	if err != nil {
		return nil, err
	}
	return res.Msg.Accounts, nil
}

type entryJSON struct {
	Seq         uint64 `json:"seq"`
	TransferID  string `json:"transfer_id"`
	Direction   string `json:"direction"`
	AmountMinor string `json:"amount_minor"`
	Asset       string `json:"asset"`
	At          string `json:"at"`
}

func (a *API) listActivity(w http.ResponseWriter, r *http.Request, s Session) {
	account := r.PathValue("account")
	after, err := uintQuery(r, "after", 0, 1<<62)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_argument", err.Error()))
		return
	}
	limit, err := uintQuery(r, "limit", 50, 500)
	if err != nil || limit == 0 {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_argument", "limit must be 1 to 500"))
		return
	}
	res, err := a.Services.Consolidated.GetActivity(r.Context(), connect.NewRequest(&consolidatedv1.GetActivityRequest{
		PartyId: s.Party, AccountId: account, AfterSeq: after, Limit: uint32(limit),
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	entries := make([]entryJSON, 0, len(res.Msg.Entries))
	for _, e := range res.Msg.Entries {
		entries = append(entries, entryJSON{
			Seq: e.Seq, TransferID: e.TransferId, Direction: e.Direction, AmountMinor: e.AmountMinor, Asset: e.Asset, At: e.At,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "next_after": res.Msg.NextAfterSeq})
}

// ---- payments ----

type paymentRequest struct {
	AccountID       string `json:"account_id"`
	AmountMinor     string `json:"amount_minor"`
	Asset           string `json:"asset"`
	BeneficiaryName string `json:"beneficiary_name"`
	AccountNumber   string `json:"account_number"`
	BankCode        string `json:"bank_code"`
	Reference       string `json:"reference"`
}

func (a *API) createPayment(w http.ResponseWriter, r *http.Request, s Session) {
	key, ok := idempotencyKey(w, r, s)
	if !ok {
		return
	}
	var req paymentRequest
	if !decode(w, r, &req) {
		return
	}
	// The source account must be the session's. Payments is asked only after that is confirmed here.
	if err := a.ownsAccount(r.Context(), s.Party, req.AccountID, req.Asset); err != nil {
		writeErr(w, err)
		return
	}
	res, err := a.Services.Payments.CreatePayment(r.Context(), connect.NewRequest(&paymentsv1.CreatePaymentRequest{
		IdempotencyKey: key, PartyId: s.Party, FromAccountId: req.AccountID, AmountMinor: req.AmountMinor,
		Asset: req.Asset, BeneficiaryName: req.BeneficiaryName, AccountNumber: req.AccountNumber,
		BankCode: req.BankCode, Reference: req.Reference, Actor: s.Actor,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"payment_id": res.Msg.PaymentId})
}

func (a *API) getPayment(w http.ResponseWriter, r *http.Request, s Session) {
	id := r.PathValue("payment")
	res, err := a.Services.Payments.GetPayment(r.Context(), connect.NewRequest(&paymentsv1.GetPaymentRequest{PaymentId: id}))
	if err != nil {
		writeErr(w, err)
		return
	}
	st := res.Msg.Status
	if st == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "payment not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"payment_id": st.PaymentId, "state": st.State.String(), "reason": st.Reason,
		"amount_minor": st.AmountMinor, "asset": st.Asset,
	})
}

// ---- fx ----

type quoteRequest struct {
	SellAsset       string `json:"sell_asset"`
	BuyAsset        string `json:"buy_asset"`
	SellAmountMinor string `json:"sell_amount_minor"`
}

func (a *API) quoteFX(w http.ResponseWriter, r *http.Request, s Session) {
	var req quoteRequest
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.FX.Quote(r.Context(), connect.NewRequest(&fxv1.QuoteRequest{
		PartyId: s.Party, SellAsset: req.SellAsset, BuyAsset: req.BuyAsset, SellAmountMinor: req.SellAmountMinor,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	q := res.Msg
	writeJSON(w, http.StatusOK, map[string]any{
		"quote_id": q.QuoteId, "sell_asset": q.SellAsset, "buy_asset": q.BuyAsset,
		"sell_amount_minor": q.SellAmountMinor, "buy_amount_minor": q.BuyAmountMinor,
		"rate_scaled": q.RateScaled, "mid_rate_scaled": q.MidRateScaled, "spread_bps": q.SpreadBps,
		"spread_minor": q.SpreadMinor, "disclosure": q.Disclosure, "expires_at": q.ExpiresAt,
	})
}

type bookRequest struct {
	QuoteID       string `json:"quote_id"`
	SellAccountID string `json:"sell_account_id"`
	BuyAccountID  string `json:"buy_account_id"`
}

func (a *API) bookFX(w http.ResponseWriter, r *http.Request, s Session) {
	var req bookRequest
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.FX.Book(r.Context(), connect.NewRequest(&fxv1.BookRequest{
		PartyId: s.Party, QuoteId: req.QuoteID, SellAccountId: req.SellAccountID, BuyAccountId: req.BuyAccountID,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"deal_id": res.Msg.DealId, "state": res.Msg.State})
}

func (a *API) getDeal(w http.ResponseWriter, r *http.Request, s Session) {
	res, err := a.Services.FX.GetDeal(r.Context(), connect.NewRequest(&fxv1.GetDealRequest{
		PartyId: s.Party, DealId: r.PathValue("deal"),
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deal_id": res.Msg.DealId, "state": res.Msg.State, "reason": res.Msg.Reason})
}

// ---- payout batches ----

type itemRequest struct {
	ItemKey         string `json:"item_key"`
	AmountMinor     string `json:"amount_minor"`
	BeneficiaryName string `json:"beneficiary_name"`
	AccountNumber   string `json:"account_number"`
	BankCode        string `json:"bank_code"`
	Reference       string `json:"reference"`
}

type batchRequest struct {
	FromAccountID string        `json:"from_account_id"`
	Asset         string        `json:"asset"`
	Items         []itemRequest `json:"items"`
}

func (a *API) createBatch(w http.ResponseWriter, r *http.Request, s Session) {
	key, ok := idempotencyKey(w, r, s)
	if !ok {
		return
	}
	var req batchRequest
	if !decode(w, r, &req) {
		return
	}
	items := make([]*payoutsv1.ItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &payoutsv1.ItemInput{
			ItemKey: it.ItemKey, AmountMinor: it.AmountMinor, BeneficiaryName: it.BeneficiaryName,
			AccountNumber: it.AccountNumber, BankCode: it.BankCode, Reference: it.Reference,
		})
	}
	res, err := a.Services.Payouts.CreateBatch(r.Context(), connect.NewRequest(&payoutsv1.CreateBatchRequest{
		IdempotencyKey: key, PartyId: s.Party, FromAccountId: req.FromAccountID, Asset: req.Asset, Items: items, Actor: s.Actor,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, batchJSON(res.Msg.Batch))
}

func (a *API) getBatch(w http.ResponseWriter, r *http.Request, s Session) {
	res, err := a.Services.Payouts.GetBatch(r.Context(), connect.NewRequest(&payoutsv1.GetBatchRequest{BatchId: r.PathValue("batch")}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, batchJSON(res.Msg.Batch))
}

func (a *API) approveBatch(w http.ResponseWriter, r *http.Request, s Session) {
	res, err := a.Services.Payouts.ApproveBatch(r.Context(), connect.NewRequest(&payoutsv1.ApproveBatchRequest{
		BatchId: r.PathValue("batch"), Actor: s.Actor,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, batchJSON(res.Msg.Batch))
}

func batchJSON(b *payoutsv1.Batch) map[string]any {
	items := make([]map[string]any, 0, len(b.Items))
	for _, it := range b.Items {
		items = append(items, map[string]any{
			"item_key": it.ItemKey, "amount_minor": it.AmountMinor, "state": it.State.String(),
			"payment_id": it.PaymentId, "reason": it.Reason,
		})
	}
	return map[string]any{
		"batch_id": b.BatchId, "from_account_id": b.FromAccountId, "asset": b.Asset, "total_minor": b.TotalMinor,
		"state": b.State.String(), "created_by": b.CreatedBy, "approved_by": b.ApprovedBy, "run": b.Run, "items": items,
	}
}

// ---- shared helpers ----

// ownsAccount confirms the account belongs to the party and holds the asset. Downstream services check too; this stops
// a request for someone else's account before it reaches a payment.
func (a *API) ownsAccount(ctx context.Context, party, account, asset string) error {
	accounts, err := a.balances(ctx, party)
	if err != nil {
		return err
	}
	for _, x := range accounts {
		if x.AccountId == account {
			if x.Asset != asset {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("the account does not hold "+asset))
			}
			return nil
		}
	}
	return connect.NewError(connect.CodePermissionDenied, errors.New("the account does not belong to you"))
}

// idempotencyKey reads the client's key and scopes it to the party, so two parties' keys cannot collide downstream.
func idempotencyKey(w http.ResponseWriter, r *http.Request, s Session) (string, bool) {
	k := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if k == "" || len(k) > 100 {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_argument", "Idempotency-Key header is required, up to 100 characters"))
		return "", false
	}
	return s.Party + ":" + k, true
}

// decode reads a JSON body strictly. An unknown field, such as a party_id sent by a client, is refused.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_argument", "the request body is not valid for this route"))
		return false
	}
	return true
}

func uintQuery(r *http.Request, name string, def, max uint64) (uint64, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil || n > max {
		return 0, errors.New(name + " is out of range")
	}
	return n, nil
}

var httpStatus = map[connect.Code]int{
	connect.CodeInvalidArgument:    http.StatusBadRequest,
	connect.CodeNotFound:           http.StatusNotFound,
	connect.CodeAlreadyExists:      http.StatusConflict,
	connect.CodeFailedPrecondition: http.StatusConflict,
	connect.CodePermissionDenied:   http.StatusForbidden,
	connect.CodeUnauthenticated:    http.StatusUnauthorized,
	connect.CodeUnavailable:        http.StatusServiceUnavailable,
}

func writeErr(w http.ResponseWriter, err error) {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		writeJSON(w, http.StatusInternalServerError, errBody("internal", "internal error"))
		return
	}
	status, ok := httpStatus[ce.Code()]
	if !ok {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, errBody(strings.ReplaceAll(ce.Code().String(), "-", "_"), ce.Message()))
}

func errBody(code, msg string) map[string]any {
	return map[string]any{"error": map[string]string{"code": code, "message": msg}}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ValidParty reports whether a party id is a canonical UUID. Authenticators use it on the header they read.
func ValidParty(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
