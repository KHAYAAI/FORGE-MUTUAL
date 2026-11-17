package api

import (
	"net/http"

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
