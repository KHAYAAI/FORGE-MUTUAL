package api

import (
	"encoding/csv"
	"net/http"
	"strconv"

	"connectrpc.com/connect"

	treasuryv1 "github.com/khayaai/forge-mutual/pkg/api/forge/treasury/v1"
)

// prudentialReport is the aggregate risk view (Banks Act regulations: management information): capital, leverage,
// liquidity and stress results. Any staff role may read it; nobody can change it here.
func (a *API) prudentialReport(w http.ResponseWriter, r *http.Request, _ Session) {
	if a.Services.Treasury == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "the prudential report is not configured"))
		return
	}
	res, err := a.Services.Treasury.GetPrudentialReport(r.Context(), connect.NewRequest(&treasuryv1.GetPrudentialReportRequest{}))
	if err != nil {
		writeErr(w, err)
		return
	}
	b, err := creditJSON.Marshal(res.Msg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal", "internal error"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// trialBalance is the management accounts' base: the trial balance, each ledger's double-entry check, and the balance
// sheet in rand.
func (a *API) trialBalance(w http.ResponseWriter, r *http.Request, _ Session) {
	if a.Services.Treasury == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "the management accounts are not configured"))
		return
	}
	res, err := a.Services.Treasury.GetTrialBalance(r.Context(), connect.NewRequest(&treasuryv1.GetTrialBalanceRequest{}))
	if err != nil {
		writeErr(w, err)
		return
	}
	b, err := creditJSON.Marshal(res.Msg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal", "internal error"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// singleCustomerView downloads the deposit insurance single customer view as CSV. It names parties by id only.
func (a *API) singleCustomerView(w http.ResponseWriter, r *http.Request, _ Session) {
	if a.Services.Treasury == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "the single customer view is not configured"))
		return
	}
	res, err := a.Services.Treasury.GetSingleCustomerView(r.Context(), connect.NewRequest(&treasuryv1.GetSingleCustomerViewRequest{}))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="single-customer-view.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"party_id", "accounts", "insured_eligible_minor", "covered_minor", "uncovered_minor", "cover_limit_minor", "as_of"})
	for _, d := range res.Msg.Depositors {
		_ = cw.Write([]string{d.PartyId, strconv.Itoa(int(d.Accounts)), d.InsuredEligibleMinor, d.CoveredMinor, d.UncoveredMinor, res.Msg.CoverLimitMinor, res.Msg.AsOf})
	}
	cw.Flush()
}
