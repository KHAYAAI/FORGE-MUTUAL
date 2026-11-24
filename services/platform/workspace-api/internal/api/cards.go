package api

import (
	"net/http"

	"connectrpc.com/connect"

	cardsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/cards/v1"
	lendingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1"
	offersv1 "github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1"
)

// Virtual debit cards for the session's party. The party always comes from the session, never from the request, so a customer can
// only ever reach their own cards. Card numbers never pass through here: DisplayCard returns the processor's own address.

func (a *API) cardsOffered(w http.ResponseWriter) bool {
	if a.Services.Cards == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "cards are not offered here"))
		return false
	}
	return true
}

func (a *API) listCards(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.cardsOffered(w) {
		return
	}
	res, err := a.Services.Cards.ListCards(r.Context(), connect.NewRequest(&cardsv1.ListCardsRequest{PartyId: s.Party}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

func (a *API) issueCard(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.cardsOffered(w) {
		return
	}
	key, ok := idemKey(w, r)
	if !ok {
		return
	}
	var req struct {
		AccountID string `json:"account_id"`
		Label     string `json:"label"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Cards.IssueCard(r.Context(), connect.NewRequest(&cardsv1.IssueCardRequest{
		IdempotencyKey: key, PartyId: s.Party, AccountId: req.AccountID, Label: req.Label}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusCreated, res.Msg.Card)
}

func (a *API) getCard(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.cardsOffered(w) {
		return
	}
	res, err := a.Services.Cards.GetCard(r.Context(), connect.NewRequest(&cardsv1.GetCardRequest{PartyId: s.Party, CardId: r.PathValue("card")}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg.Card)
}

func (a *API) setCardStatus(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.cardsOffered(w) {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Cards.SetCardStatus(r.Context(), connect.NewRequest(&cardsv1.SetCardStatusRequest{
		PartyId: s.Party, CardId: r.PathValue("card"), Status: req.Status, Actor: s.Actor}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg.Card)
}

func (a *API) setCardLimits(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.cardsOffered(w) {
		return
	}
	var req struct {
		PerTransactionLimitMinor string `json:"per_transaction_limit_minor"`
		DailyLimitMinor          string `json:"daily_limit_minor"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Cards.SetLimits(r.Context(), connect.NewRequest(&cardsv1.SetLimitsRequest{
		PartyId: s.Party, CardId: r.PathValue("card"), PerTransactionLimitMinor: req.PerTransactionLimitMinor, DailyLimitMinor: req.DailyLimitMinor}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg.Card)
}

func (a *API) listCardAuthorisations(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.cardsOffered(w) {
		return
	}
	res, err := a.Services.Cards.ListAuthorisations(r.Context(), connect.NewRequest(&cardsv1.ListAuthorisationsRequest{PartyId: s.Party, CardId: r.PathValue("card")}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

func (a *API) displayCard(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.cardsOffered(w) {
		return
	}
	res, err := a.Services.Cards.DisplayCard(r.Context(), connect.NewRequest(&cardsv1.DisplayCardRequest{PartyId: s.Party, CardId: r.PathValue("card")}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

// Third-party offers: investments and crypto-asset services that a licensed provider supplies. Every offer is returned with the
// third-party statement the offers service generated; this layer never writes, edits or drops it.

func (a *API) offersOffered(w http.ResponseWriter) bool {
	if a.Services.Offers == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "third-party offers are not offered here"))
		return false
	}
	return true
}

func (a *API) listOffers(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.offersOffered(w) {
		return
	}
	res, err := a.Services.Offers.ListOffers(r.Context(), connect.NewRequest(&offersv1.ListOffersRequest{PartyId: s.Party, Category: r.URL.Query().Get("category")}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

func (a *API) createReferral(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.offersOffered(w) {
		return
	}
	key, ok := idemKey(w, r)
	if !ok {
		return
	}
	var req struct {
		AcknowledgedThirdParty bool   `json:"acknowledged_third_party"`
		StatementHash          string `json:"statement_hash"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Offers.CreateReferral(r.Context(), connect.NewRequest(&offersv1.CreateReferralRequest{
		IdempotencyKey: key, PartyId: s.Party, OfferId: r.PathValue("offer"), AcknowledgedThirdParty: req.AcknowledgedThirdParty, StatementHash: req.StatementHash}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusCreated, res.Msg.Referral)
}

func (a *API) listReferrals(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.offersOffered(w) {
		return
	}
	res, err := a.Services.Offers.ListReferrals(r.Context(), connect.NewRequest(&offersv1.ListReferralsRequest{PartyId: s.Party}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}

// preAgreement is the statement and quotation the customer must acknowledge before accepting a credit offer (NCA s.92).
func (a *API) preAgreement(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.lendingOffered(w) {
		return
	}
	res, err := a.Services.Lending.GetPreAgreement(r.Context(), connect.NewRequest(&lendingv1.GetPreAgreementRequest{PartyId: s.Party, FacilityId: r.PathValue("facility")}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProto(w, http.StatusOK, res.Msg)
}
