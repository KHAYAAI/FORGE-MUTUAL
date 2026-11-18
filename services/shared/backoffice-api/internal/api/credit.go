package api

import (
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	lendingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1"
)

// The credit desk. A credit officer sees applications with their assessment and decides them; the officer is the
// session's actor, so no one can decide in another's name, and lending refuses an applicant deciding their own.

var creditJSON = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}

func (a *API) creditOffered(w http.ResponseWriter) bool {
	if a.Services.Credit == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "lending is not offered here"))
		return false
	}
	return true
}

func (a *API) listCreditApplications(w http.ResponseWriter, r *http.Request, _ Session) {
	if !a.creditOffered(w) {
		return
	}
	res, err := a.Services.Credit.ListCreditApplications(r.Context(), connect.NewRequest(&lendingv1.ListCreditApplicationsRequest{Status: r.URL.Query().Get("status")}))
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

type creditDecision struct {
	Approve       bool   `json:"approve"`
	ApprovedMinor string `json:"approved_minor"`
	RateBps       uint32 `json:"rate_bps"`
	Note          string `json:"note"`
}

func (a *API) decideCredit(w http.ResponseWriter, r *http.Request, s Session) {
	if !a.creditOffered(w) {
		return
	}
	var req creditDecision
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Credit.DecideCreditApplication(r.Context(), connect.NewRequest(&lendingv1.DecideCreditApplicationRequest{
		FacilityId: r.PathValue("facility"), Approve: req.Approve, ApprovedMinor: req.ApprovedMinor, RateBps: req.RateBps,
		Note: req.Note, Officer: s.Actor,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	b, _ := creditJSON.Marshal(res.Msg.Facility)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
