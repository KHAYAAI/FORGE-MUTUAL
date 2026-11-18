package api

import (
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	depositsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/deposits/v1"
)

// Savings and notice deposits. The party is the session's; nothing in a request names one.

var protoJSON = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}

// writeProto writes a response message as JSON with snake_case names, matching the rest of this API.
func writeProto(w http.ResponseWriter, status int, m proto.Message) {
	b, err := protoJSON.Marshal(m)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal", "internal error"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func (a *API) depositsOffered(w http.ResponseWriter) bool {
	if a.Services.Deposits == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "savings are not offered here"))
		return false
	}
	return true
}

func (a *API) listDepositProducts(w http.ResponseWriter, r *http.Request, _ Session) {
	if !a.depositsOffered(w) {
		return
	}
	res, err := a.Services.Deposits.ListProducts(r.Context(), connect.NewRequest(&depositsv1.ListProductsRequest{}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

func (a *API) listDepositAccounts(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.depositsOffered(w) {
		return
	}
	res, err := a.Services.Deposits.ListDepositAccounts(r.Context(), connect.NewRequest(&depositsv1.ListDepositAccountsRequest{PartyId: s.Party}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

func (a *API) openDepositAccount(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.depositsOffered(w) {
		return
	}
	var req struct {
		ProductID string `json:"product_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Deposits.OpenDepositAccount(r.Context(), connect.NewRequest(&depositsv1.OpenDepositAccountRequest{
		PartyId: s.Party, ProductId: req.ProductID, Actor: s.Actor,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusCreated, res.Msg.Account)
}

type depositMove struct {
	FromAccountID string `json:"from_account_id"`
	ToAccountID   string `json:"to_account_id"`
	AmountMinor   string `json:"amount_minor"`
}

func (a *API) moveDeposit(withdraw bool) func(http.ResponseWriter, *http.Request, Session) {
	return func(w http.ResponseWriter, r *http.Request, s Session) {
		if !a.depositsOffered(w) {
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if key == "" || len(key) > 100 {
			writeJSON(w, http.StatusBadRequest, errBody("invalid_argument", "an Idempotency-Key header of up to 100 characters is required"))
			return
		}
		var req depositMove
		if !decode(w, r, &req) {
			return
		}
		if withdraw {
			res, err := a.Services.Deposits.Withdraw(r.Context(), connect.NewRequest(&depositsv1.WithdrawRequest{
				IdempotencyKey: key, PartyId: s.Party, FromAccountId: req.FromAccountID, ToAccountId: req.ToAccountID,
				AmountMinor: req.AmountMinor, Actor: s.Actor,
			}))
			if err != nil {
				writeErr(w, err)
				return
			}
			writeProto(w, http.StatusOK, res.Msg)
			return
		}
		res, err := a.Services.Deposits.Deposit(r.Context(), connect.NewRequest(&depositsv1.DepositRequest{
			IdempotencyKey: key, PartyId: s.Party, FromAccountId: req.FromAccountID, ToAccountId: req.ToAccountID,
			AmountMinor: req.AmountMinor, Actor: s.Actor,
		}))
		if err != nil {
			writeErr(w, err)
			return
		}
		writeProto(w, http.StatusOK, res.Msg)
	}
}

func (a *API) cancelNotice(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.depositsOffered(w) {
		return
	}
	res, err := a.Services.Deposits.CancelNotice(r.Context(), connect.NewRequest(&depositsv1.CancelNoticeRequest{
		PartyId: s.Party, NoticeId: r.PathValue("notice"), Actor: s.Actor,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg.Notice)
}
