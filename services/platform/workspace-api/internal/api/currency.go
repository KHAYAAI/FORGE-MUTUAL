package api

import (
	"net/http"

	"connectrpc.com/connect"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
)

// Currency accounts and cross-border credits. The party is always the session's; nothing in a request names one.

type openAccountRequest struct {
	Asset string `json:"asset"`
}

func (a *API) openAccount(w http.ResponseWriter, r *http.Request, s Session) {
	var req openAccountRequest
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Onboarding.OpenCurrencyAccount(r.Context(), connect.NewRequest(&onboardingv1.OpenCurrencyAccountRequest{
		PartyId: s.Party, Asset: req.Asset, Actor: s.Actor,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"account_id": res.Msg.AccountId, "asset": res.Msg.Asset, "reference": res.Msg.Reference, "partner": res.Msg.Partner,
	})
}

func (a *API) receivingDetails(w http.ResponseWriter, r *http.Request, s Session) {
	res, err := a.Services.References.ListInboundReferences(r.Context(), connect.NewRequest(&paymentsv1.ListInboundReferencesRequest{PartyId: s.Party}))
	if err != nil {
		writeErr(w, err)
		return
	}
	out := []map[string]string{}
	for _, ref := range res.Msg.References {
		out = append(out, map[string]string{"reference": ref.Reference, "asset": ref.Asset, "account_id": ref.AccountId})
	}
	writeJSON(w, http.StatusOK, map[string]any{"details": out})
}

func creditJSON(c *paymentsv1.CrossBorderCredit) map[string]string {
	return map[string]string{
		"credit_id": c.CreditId, "account_id": c.AccountId, "asset": c.Asset, "amount_minor": c.AmountMinor,
		"reference": c.Reference, "booked_at": c.BookedAt, "status": c.Status, "reason_code": c.ReasonCode,
		"reason_note": c.ReasonNote, "classified_by": c.ClassifiedBy, "classified_at": c.ClassifiedAt,
	}
}

func (a *API) listCrossBorder(w http.ResponseWriter, r *http.Request, s Session) {
	res, err := a.Services.CrossBorder.ListCrossBorderCredits(r.Context(), connect.NewRequest(&paymentsv1.ListCrossBorderCreditsRequest{PartyId: s.Party}))
	if err != nil {
		writeErr(w, err)
		return
	}
	out := []map[string]string{}
	for _, c := range res.Msg.Credits {
		out = append(out, creditJSON(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"credits": out})
}

type reasonRequest struct {
	ReasonCode string `json:"reason_code"`
	Note       string `json:"note"`
}

func (a *API) classifyCrossBorder(w http.ResponseWriter, r *http.Request, s Session) {
	var req reasonRequest
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.CrossBorder.ClassifyCrossBorderCredit(r.Context(), connect.NewRequest(&paymentsv1.ClassifyCrossBorderCreditRequest{
		PartyId: s.Party, CreditId: r.PathValue("credit"), ReasonCode: req.ReasonCode, Note: req.Note, Actor: s.Actor,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, creditJSON(res.Msg.Credit))
}

func (a *API) listReasons(w http.ResponseWriter, r *http.Request, _ Session) {
	res, err := a.Services.CrossBorder.ListPaymentReasons(r.Context(), connect.NewRequest(&paymentsv1.ListPaymentReasonsRequest{}))
	if err != nil {
		writeErr(w, err)
		return
	}
	out := []map[string]string{}
	for _, p := range res.Msg.Reasons {
		out = append(out, map[string]string{"code": p.Code, "label": p.Label})
	}
	writeJSON(w, http.StatusOK, map[string]any{"reasons": out})
}
