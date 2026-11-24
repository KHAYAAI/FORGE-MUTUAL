package api

import (
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	offersv1 "github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1"
)

// Third-party offers, compliance's side. Compliance registers a provider, a DIFFERENT person verifies its licence on the regulator's
// own register, one person writes an offer and another switches it on. The actor is always the session's, so no one acts in another's
// name; the offers service enforces the two-person rules and every statement customers see.

var partnersJSON = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}

func (a *API) partnersOffered(w http.ResponseWriter) bool {
	if a.Services.Partners == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "third-party offers are not offered here"))
		return false
	}
	return true
}

func writePartners(w http.ResponseWriter, status int, m proto.Message) {
	b, err := partnersJSON.Marshal(m)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal", "internal error"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func (a *API) listProviders(w http.ResponseWriter, r *http.Request, _ Session) {
	if !a.partnersOffered(w) {
		return
	}
	res, err := a.Services.Partners.ListProviders(r.Context(), connect.NewRequest(&offersv1.ListProvidersRequest{}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writePartners(w, http.StatusOK, res.Msg)
}

func (a *API) listPartnerOffers(w http.ResponseWriter, r *http.Request, _ Session) {
	if !a.partnersOffered(w) {
		return
	}
	res, err := a.Services.Partners.ListAdminOffers(r.Context(), connect.NewRequest(&offersv1.ListAdminOffersRequest{}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writePartners(w, http.StatusOK, res.Msg)
}

func (a *API) registerProvider(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.partnersOffered(w) {
		return
	}
	var req struct {
		ProviderID       string `json:"provider_id"`
		Name             string `json:"name"`
		LicenceType      string `json:"licence_type"`
		Regulator        string `json:"regulator"`
		LicenceNumber    string `json:"licence_number"`
		LicenceSourceURL string `json:"licence_source_url"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Partners.RegisterProvider(r.Context(), connect.NewRequest(&offersv1.RegisterProviderRequest{
		ProviderId: req.ProviderID, Name: req.Name, LicenceType: req.LicenceType, Regulator: req.Regulator, LicenceNumber: req.LicenceNumber,
		LicenceSourceUrl: req.LicenceSourceURL, Actor: s.Actor}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writePartners(w, http.StatusCreated, res.Msg.Provider)
}

func (a *API) verifyLicence(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.partnersOffered(w) {
		return
	}
	var req struct {
		LicenceExpiresAt string `json:"licence_expires_at"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Partners.VerifyLicence(r.Context(), connect.NewRequest(&offersv1.VerifyLicenceRequest{
		ProviderId: r.PathValue("provider"), Actor: s.Actor, LicenceExpiresAt: req.LicenceExpiresAt}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writePartners(w, http.StatusOK, res.Msg.Provider)
}

func (a *API) suspendProvider(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.partnersOffered(w) {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Partners.SuspendProvider(r.Context(), connect.NewRequest(&offersv1.SuspendProviderRequest{
		ProviderId: r.PathValue("provider"), Reason: req.Reason, Actor: s.Actor}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writePartners(w, http.StatusOK, res.Msg.Provider)
}

func (a *API) upsertPartnerOffer(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.partnersOffered(w) {
		return
	}
	var req struct {
		OfferID       string `json:"offer_id"`
		ProviderID    string `json:"provider_id"`
		Category      string `json:"category"`
		Title         string `json:"title"`
		Summary       string `json:"summary"`
		HandoffURL    string `json:"handoff_url"`
		FeeDisclosure string `json:"fee_disclosure"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Partners.UpsertOffer(r.Context(), connect.NewRequest(&offersv1.UpsertOfferRequest{
		OfferId: req.OfferID, ProviderId: req.ProviderID, Category: req.Category, Title: req.Title, Summary: req.Summary,
		HandoffUrl: req.HandoffURL, FeeDisclosure: req.FeeDisclosure, Actor: s.Actor}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writePartners(w, http.StatusOK, res.Msg.Offer)
}

func (a *API) setPartnerOfferStatus(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.partnersOffered(w) {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Partners.SetOfferStatus(r.Context(), connect.NewRequest(&offersv1.SetOfferStatusRequest{
		OfferId: r.PathValue("offer"), Status: req.Status, Actor: s.Actor}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writePartners(w, http.StatusOK, res.Msg)
}
