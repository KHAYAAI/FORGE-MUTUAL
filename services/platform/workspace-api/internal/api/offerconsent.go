package api

import (
	"net/http"

	"connectrpc.com/connect"

	consentv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1"
	offersv1 "github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1"
)

// The customer's consent to share their details with a third-party provider. A referral needs it (the offers service checks), and it is
// the customer's to give and to withdraw. It is recorded in the consent service, tied to the exact statement the customer saw.
//
// What the customer is told is shared is the offer's own Shares list, the very list the offers service's sender builds the details from, so
// the words on screen and the data sent cannot drift apart.
//
// The consent service accepts only plain purposes, so the provider is in the scope: purpose "share_with_partner", scope "referral:<provider>".
// (These must match partner-offers' ConsentPurpose and ConsentScope; a test pins them.)
const (
	partnerConsentPurpose = "share_with_partner"
	partnerConsentScope   = "referral:"
)

// offerFor finds an offer the session party may currently see. Hidden offers look absent, so a consent can only ever be given for a
// provider the customer was actually shown.
func (a *API) offerFor(w http.ResponseWriter, r *http.Request, s Session) (*offersv1.Offer, bool) {
	if !a.offersOffered(w) {
		return nil, false
	}
	if a.Services.Consent == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "consent is not offered here"))
		return nil, false
	}
	res, err := a.Services.Offers.ListOffers(r.Context(), connect.NewRequest(&offersv1.ListOffersRequest{PartyId: s.Party}))
	if err != nil {
		writeErr(w, err)
		return nil, false
	}
	for _, o := range res.Msg.Offers {
		if o.OfferId == r.PathValue("offer") {
			return o, true
		}
	}
	writeJSON(w, http.StatusNotFound, errBody("not_found", "no such offer"))
	return nil, false
}

type consentView struct {
	Granted      bool     `json:"granted"`
	ConsentID    string   `json:"consent_id"`
	ProviderName string   `json:"provider_name"`
	Shares       []string `json:"shares"`
}

func (a *API) currentConsent(r *http.Request, s Session, o *offersv1.Offer) (consentView, error) {
	c, err := a.Services.Consent.Check(r.Context(), connect.NewRequest(&consentv1.CheckRequest{PartyId: s.Party, Purpose: partnerConsentPurpose, Scope: partnerConsentScope + o.ProviderId}))
	if err != nil {
		return consentView{}, err
	}
	return consentView{Granted: c.Msg.Allowed, ConsentID: c.Msg.ConsentId, ProviderName: o.ProviderName, Shares: o.Shares}, nil
}

func (a *API) getOfferConsent(w http.ResponseWriter, r *http.Request, s Session) {
	o, ok := a.offerFor(w, r, s)
	if !ok {
		return
	}
	v, err := a.currentConsent(r, s, o)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) grantOfferConsent(w http.ResponseWriter, r *http.Request, s Session) {
	o, ok := a.offerFor(w, r, s)
	if !ok {
		return
	}
	key, ok := idemKey(w, r)
	if !ok {
		return
	}
	var req struct {
		Agrees        bool   `json:"agrees"`
		StatementHash string `json:"statement_hash"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !req.Agrees || req.StatementHash != o.StatementHash {
		writeJSON(w, http.StatusConflict, errBody("failed_precondition", "you must agree, and have read the current statement for this product, to give your consent"))
		return
	}
	if _, err := a.Services.Consent.Grant(r.Context(), connect.NewRequest(&consentv1.GrantRequest{
		IdempotencyKey: key, PartyId: s.Party, Purpose: partnerConsentPurpose, Scopes: []string{partnerConsentScope + o.ProviderId},
		DataSources: []string{"identity"}, EvidenceRef: "offer:" + o.OfferId + ";statement:" + o.StatementHash, Actor: s.Actor})); err != nil {
		writeErr(w, err)
		return
	}
	v, err := a.currentConsent(r, s, o)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (a *API) withdrawOfferConsent(w http.ResponseWriter, r *http.Request, s Session) {
	o, ok := a.offerFor(w, r, s)
	if !ok {
		return
	}
	v, err := a.currentConsent(r, s, o)
	if err != nil {
		writeErr(w, err)
		return
	}
	if v.Granted {
		if _, err := a.Services.Consent.Revoke(r.Context(), connect.NewRequest(&consentv1.RevokeRequest{ConsentId: v.ConsentID, Actor: s.Actor})); err != nil {
			writeErr(w, err)
			return
		}
		if v, err = a.currentConsent(r, s, o); err != nil {
			writeErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, v)
}
