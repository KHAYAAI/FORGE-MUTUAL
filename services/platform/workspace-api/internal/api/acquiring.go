package api

import (
	"net/http"

	"connectrpc.com/connect"

	acquiringv1 "github.com/khayaai/forge-mutual/pkg/api/forge/acquiring/v1"
)

// Card acceptance. The merchant's routes need a session; the payer's checkout does not: the link's token, which only
// the merchant and their payer have, is the capability, and it shows nothing but what is being paid and to whom.

func (a *API) acquiringOffered(w http.ResponseWriter) bool {
	if a.Services.Acquiring == nil || a.Services.Checkout == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "card payments are not offered here"))
		return false
	}
	return true
}

func (a *API) createPaymentLink(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.acquiringOffered(w) {
		return
	}
	key, ok := idemKey(w, r)
	if !ok {
		return
	}
	var req struct {
		SettlementAccountID string `json:"settlement_account_id"`
		AmountMinor         string `json:"amount_minor"`
		Description         string `json:"description"`
		Reference           string `json:"reference"`
		Channel             string `json:"channel"`
		MerchantName        string `json:"merchant_name"`
		ValidHours          uint32 `json:"valid_hours"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Acquiring.CreatePaymentLink(r.Context(), connect.NewRequest(&acquiringv1.CreatePaymentLinkRequest{
		IdempotencyKey: key, PartyId: s.Party, SettlementAccountId: req.SettlementAccountID, AmountMinor: req.AmountMinor,
		Description: req.Description, Reference: req.Reference, Channel: req.Channel, MerchantName: req.MerchantName,
		ValidHours: req.ValidHours, Actor: s.Actor,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusCreated, res.Msg.Link)
}

func (a *API) listPaymentLinks(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.acquiringOffered(w) {
		return
	}
	res, err := a.Services.Acquiring.ListPaymentLinks(r.Context(), connect.NewRequest(&acquiringv1.ListPaymentLinksRequest{PartyId: s.Party}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

func (a *API) cancelPaymentLink(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.acquiringOffered(w) {
		return
	}
	res, err := a.Services.Acquiring.CancelPaymentLink(r.Context(), connect.NewRequest(&acquiringv1.CancelPaymentLinkRequest{
		PartyId: s.Party, LinkId: r.PathValue("link"), Actor: s.Actor}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg.Link)
}

func (a *API) refundPayment(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.acquiringOffered(w) {
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
	res, err := a.Services.Acquiring.RefundPayment(r.Context(), connect.NewRequest(&acquiringv1.RefundPaymentRequest{
		IdempotencyKey: key, PartyId: s.Party, LinkId: r.PathValue("link"), AmountMinor: req.AmountMinor, Actor: s.Actor}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg.Link)
}

func (a *API) listSettlements(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.acquiringOffered(w) {
		return
	}
	res, err := a.Services.Acquiring.ListSettlements(r.Context(), connect.NewRequest(&acquiringv1.ListSettlementsRequest{PartyId: s.Party}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

// --- the payer's checkout: no session ---

func (a *API) getCheckout(w http.ResponseWriter, r *http.Request) {
	if !a.acquiringOffered(w) {
		return
	}
	res, err := a.Services.Checkout.GetCheckout(r.Context(), connect.NewRequest(&acquiringv1.GetCheckoutRequest{Token: r.PathValue("token")}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

func (a *API) startCheckout(w http.ResponseWriter, r *http.Request) {
	if !a.acquiringOffered(w) {
		return
	}
	res, err := a.Services.Checkout.StartCheckout(r.Context(), connect.NewRequest(&acquiringv1.StartCheckoutRequest{Token: r.PathValue("token")}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

func (a *API) simulateCheckout(w http.ResponseWriter, r *http.Request) {
	if !a.acquiringOffered(w) {
		return
	}
	var req struct {
		Approve bool `json:"approve"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Checkout.SimulatePayment(r.Context(), connect.NewRequest(&acquiringv1.SimulatePaymentRequest{Token: r.PathValue("token"), Approve: req.Approve}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}
