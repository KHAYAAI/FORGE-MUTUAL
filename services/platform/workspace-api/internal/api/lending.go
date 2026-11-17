package api

import (
	"net/http"

	"connectrpc.com/connect"

	lendingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1"
)

// Credit facilities for the session's party. Applying, drawing and repaying take an Idempotency-Key.

func (a *API) lendingOffered(w http.ResponseWriter) bool {
	if a.Services.Lending == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "credit is not offered here"))
		return false
	}
	return true
}

func idemKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) > 100 {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_argument", "an Idempotency-Key header of up to 100 characters is required"))
		return "", false
	}
	return key, true
}

func (a *API) listLoanProducts(w http.ResponseWriter, r *http.Request, _ Session) {
	if !a.lendingOffered(w) {
		return
	}
	res, err := a.Services.Lending.ListLoanProducts(r.Context(), connect.NewRequest(&lendingv1.ListLoanProductsRequest{}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

func (a *API) listFacilities(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.lendingOffered(w) {
		return
	}
	res, err := a.Services.Lending.ListFacilities(r.Context(), connect.NewRequest(&lendingv1.ListFacilitiesRequest{PartyId: s.Party}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

func (a *API) applyForCredit(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.lendingOffered(w) {
		return
	}
	key, ok := idemKey(w, r)
	if !ok {
		return
	}
	var req struct {
		ProductID        string `json:"product_id"`
		AmountMinor      string `json:"amount_minor"`
		TermMonths       uint32 `json:"term_months"`
		CurrentAccountID string `json:"current_account_id"`
		Purpose          string `json:"purpose"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Lending.ApplyForCredit(r.Context(), connect.NewRequest(&lendingv1.ApplyForCreditRequest{
		IdempotencyKey: key, PartyId: s.Party, ProductId: req.ProductID, AmountMinor: req.AmountMinor, TermMonths: req.TermMonths,
		CurrentAccountId: req.CurrentAccountID, Purpose: req.Purpose, Actor: s.Actor,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	f := res.Msg.Facility
	f.DecisionNote = ""
	writeProto(w, http.StatusCreated, f)
}

func (a *API) acceptOffer(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.lendingOffered(w) {
		return
	}
	var req struct {
		AcceptsTerms bool `json:"accepts_terms"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Lending.AcceptOffer(r.Context(), connect.NewRequest(&lendingv1.AcceptOfferRequest{
		PartyId: s.Party, FacilityId: r.PathValue("facility"), Actor: s.Actor, AcceptsTerms: req.AcceptsTerms,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	f := res.Msg.Facility
	f.DecisionNote = ""
	writeProto(w, http.StatusOK, f)
}

func (a *API) moveCredit(repay bool) func(http.ResponseWriter, *http.Request, Session) {
	return func(w http.ResponseWriter, r *http.Request, s Session) {
		if !a.lendingOffered(w) {
			return
		}
		key, ok := idemKey(w, r)
		if !ok {
			return
		}
		var req struct {
			AmountMinor string `json:"amount_minor"`
		}
		if !decode(w, r, &req) {
			return
		}
		var f *lendingv1.Facility
		if repay {
			res, err := a.Services.Lending.Repay(r.Context(), connect.NewRequest(&lendingv1.RepayRequest{
				IdempotencyKey: key, PartyId: s.Party, FacilityId: r.PathValue("facility"), AmountMinor: req.AmountMinor, Actor: s.Actor}))
			if err != nil {
				writeErr(w, err)
				return
			}
			f = res.Msg.Facility
		} else {
			res, err := a.Services.Lending.Draw(r.Context(), connect.NewRequest(&lendingv1.DrawRequest{
				IdempotencyKey: key, PartyId: s.Party, FacilityId: r.PathValue("facility"), AmountMinor: req.AmountMinor, Actor: s.Actor}))
			if err != nil {
				writeErr(w, err)
				return
			}
			f = res.Msg.Facility
		}
		f.DecisionNote = ""
		writeProto(w, http.StatusOK, f)
	}
}
