// Package api is the back office's REST surface for staff: compliance cases, payment repair and approval, and the
// reconciliation dashboard. It keeps no state. The acting person and their roles come from the session, never from the
// request body, and a role check runs before any downstream call. Request bodies are decoded strictly.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	casesv1 "github.com/khayaai/forge-mutual/pkg/api/forge/cases/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/cases/v1/casesv1connect"
	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	reconv1 "github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1/reconv1connect"
)

// Roles a staff member can hold. A person can hold more than one.
const (
	RoleCompliance = "compliance" // closes compliance cases, as a checker
	RoleOperations = "operations" // decides payment approvals, resolves repairs, investigates reconciliation breaks
)

// Session is the signed-in staff member.
type Session struct {
	Actor string
	Roles map[string]bool
}

// Authenticator turns a request into a staff session, or refuses it.
type Authenticator interface {
	Session(r *http.Request) (Session, error)
}

// Services are the downstream clients the back office calls.
type Services struct {
	Cases    casesv1connect.CasesServiceClient
	Payments paymentsv1connect.PaymentsServiceClient
	Recon    reconv1connect.ReconServiceClient
}

// API serves the back-office routes.
type API struct {
	Auth     Authenticator
	Services Services
}

// Handler returns the routes. Health checks are open; every other route needs a staff session.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /v1/cases", a.staff(a.listCases))
	mux.HandleFunc("GET /v1/cases/{case}", a.staff(a.getCase))
	mux.HandleFunc("POST /v1/cases/{case}/close", a.staff(needRole(RoleCompliance, a.closeCase)))
	mux.HandleFunc("GET /v1/payments/{payment}", a.staff(a.getPayment))
	mux.HandleFunc("POST /v1/payments/{payment}/approval", a.staff(needRole(RoleOperations, a.decideApproval)))
	mux.HandleFunc("POST /v1/payments/{payment}/repair", a.staff(needRole(RoleOperations, a.resolveRepair)))
	mux.HandleFunc("GET /v1/recon/gate", a.staff(a.getGate))
	mux.HandleFunc("GET /v1/recon/breaks", a.staff(a.listBreaks))
	mux.HandleFunc("POST /v1/recon/breaks/{break}/investigate", a.staff(needRole(RoleOperations, a.investigate)))
	return mux
}

type handler func(w http.ResponseWriter, r *http.Request, s Session)

func (a *API) staff(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := a.Auth.Session(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errBody("unauthenticated", "sign in to continue"))
			return
		}
		h(w, r, s)
	}
}

func needRole(role string, h handler) handler {
	return func(w http.ResponseWriter, r *http.Request, s Session) {
		if !s.Roles[role] {
			writeJSON(w, http.StatusForbidden, errBody("permission_denied", "your role does not allow this action"))
			return
		}
		h(w, r, s)
	}
}

// ---- cases ----

type caseJSON struct {
	CaseID      string   `json:"case_id"`
	PartyID     string   `json:"party_id"`
	Rule        string   `json:"rule"`
	Severity    string   `json:"severity"`
	Detail      string   `json:"detail"`
	Evidence    []string `json:"evidence"`
	Status      string   `json:"status"`
	OpenedBy    string   `json:"opened_by"`
	OpenedAt    string   `json:"opened_at"`
	SLADueAt    string   `json:"sla_due_at"`
	Overdue     bool     `json:"overdue"`
	ClosedBy    string   `json:"closed_by"`
	ClosedAt    string   `json:"closed_at"`
	ClosingNote string   `json:"closing_note"`
}

func caseOut(c *casesv1.Case) caseJSON {
	return caseJSON{
		CaseID: c.CaseId, PartyID: c.PartyId, Rule: c.Rule, Severity: c.Severity, Detail: c.Detail,
		Evidence: c.Evidence, Status: c.Status, OpenedBy: c.OpenedBy, OpenedAt: c.OpenedAt, SLADueAt: c.SlaDueAt,
		Overdue: c.Overdue, ClosedBy: c.ClosedBy, ClosedAt: c.ClosedAt, ClosingNote: c.ClosingNote,
	}
}

func (a *API) listCases(w http.ResponseWriter, r *http.Request, s Session) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "open" && status != "all" {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_argument", "status must be open or all"))
		return
	}
	limit, err := uintQuery(r, "limit", 100, 500)
	if err != nil || limit == 0 {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_argument", "limit must be 1 to 500"))
		return
	}
	res, err := a.Services.Cases.ListCases(r.Context(), connect.NewRequest(&casesv1.ListCasesRequest{
		OpenOnly: status != "all", Limit: uint32(limit),
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]caseJSON, 0, len(res.Msg.Cases))
	for _, c := range res.Msg.Cases {
		out = append(out, caseOut(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"cases": out})
}

func (a *API) getCase(w http.ResponseWriter, r *http.Request, s Session) {
	res, err := a.Services.Cases.GetCase(r.Context(), connect.NewRequest(&casesv1.GetCaseRequest{CaseId: r.PathValue("case")}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, caseOut(res.Msg.Case))
}

type closeRequest struct {
	Note string `json:"note"`
}

func (a *API) closeCase(w http.ResponseWriter, r *http.Request, s Session) {
	var req closeRequest
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Cases.CloseCase(r.Context(), connect.NewRequest(&casesv1.CloseCaseRequest{
		CaseId: r.PathValue("case"), Actor: s.Actor, Note: req.Note,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, caseOut(res.Msg.Case))
}

// ---- payments ----

func (a *API) getPayment(w http.ResponseWriter, r *http.Request, s Session) {
	res, err := a.Services.Payments.GetPayment(r.Context(), connect.NewRequest(&paymentsv1.GetPaymentRequest{PaymentId: r.PathValue("payment")}))
	if err != nil {
		writeErr(w, err)
		return
	}
	st := res.Msg.Status
	if st == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "payment not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"payment_id": st.PaymentId, "state": st.State.String(), "reason": st.Reason,
		"amount_minor": st.AmountMinor, "asset": st.Asset, "initiated_by": st.InitiatedBy,
	})
}

type approvalRequest struct {
	Approve *bool `json:"approve"`
}

func (a *API) decideApproval(w http.ResponseWriter, r *http.Request, s Session) {
	var req approvalRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Approve == nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_argument", "approve is required: true or false"))
		return
	}
	id := r.PathValue("payment")
	if _, err := a.Services.Payments.ApprovePayment(r.Context(), connect.NewRequest(&paymentsv1.ApprovePaymentRequest{
		PaymentId: id, Approve: *req.Approve, Approver: s.Actor,
	})); err != nil {
		writeErr(w, err)
		return
	}
	decision := "REFUSED"
	if *req.Approve {
		decision = "APPROVED"
	}
	writeJSON(w, http.StatusOK, map[string]any{"payment_id": id, "decision": decision})
}

type repairRequest struct {
	Settled *bool `json:"settled"`
}

func (a *API) resolveRepair(w http.ResponseWriter, r *http.Request, s Session) {
	var req repairRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Settled == nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_argument", "settled is required: true if the rail confirms the money moved"))
		return
	}
	id := r.PathValue("payment")
	if _, err := a.Services.Payments.ResolveRepair(r.Context(), connect.NewRequest(&paymentsv1.ResolveRepairRequest{
		PaymentId: id, Settled: *req.Settled, Operator: s.Actor,
	})); err != nil {
		writeErr(w, err)
		return
	}
	outcome := "NOT_SETTLED"
	if *req.Settled {
		outcome = "SETTLED"
	}
	writeJSON(w, http.StatusOK, map[string]any{"payment_id": id, "outcome": outcome})
}

// ---- reconciliation ----

type breakJSON struct {
	BreakID           string `json:"break_id"`
	Asset             string `json:"asset"`
	Check             string `json:"check"`
	DifferenceMinor   string `json:"difference_minor"`
	Detail            string `json:"detail"`
	Status            string `json:"status"`
	OpenedAt          string `json:"opened_at"`
	LastSeenAt        string `json:"last_seen_at"`
	ClearedAt         string `json:"cleared_at"`
	InvestigationNote string `json:"investigation_note"`
	InvestigatedBy    string `json:"investigated_by"`
	InvestigatedAt    string `json:"investigated_at"`
}

func breakOut(b *reconv1.Break) breakJSON {
	return breakJSON{
		BreakID: b.BreakId, Asset: b.Asset, Check: b.Check, DifferenceMinor: b.DifferenceMinor, Detail: b.Detail,
		Status: b.Status, OpenedAt: b.OpenedAt, LastSeenAt: b.LastSeenAt, ClearedAt: b.ClearedAt,
		InvestigationNote: b.InvestigationNote, InvestigatedBy: b.InvestigatedBy, InvestigatedAt: b.InvestigatedAt,
	}
}

func (a *API) getGate(w http.ResponseWriter, r *http.Request, s Session) {
	res, err := a.Services.Recon.GetGate(r.Context(), connect.NewRequest(&reconv1.GetGateRequest{}))
	if err != nil {
		writeErr(w, err)
		return
	}
	reasons := res.Msg.Reasons
	if reasons == nil {
		reasons = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"break_active": res.Msg.BreakActive, "reasons": reasons, "as_of": res.Msg.AsOf,
	})
}

func (a *API) listBreaks(w http.ResponseWriter, r *http.Request, s Session) {
	openOnly := r.URL.Query().Get("open_only") != "false"
	res, err := a.Services.Recon.ListBreaks(r.Context(), connect.NewRequest(&reconv1.ListBreaksRequest{OpenOnly: openOnly}))
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]breakJSON, 0, len(res.Msg.Breaks))
	for _, b := range res.Msg.Breaks {
		out = append(out, breakOut(b))
	}
	writeJSON(w, http.StatusOK, map[string]any{"breaks": out})
}

type investigateRequest struct {
	Note string `json:"note"`
}

func (a *API) investigate(w http.ResponseWriter, r *http.Request, s Session) {
	var req investigateRequest
	if !decode(w, r, &req) {
		return
	}
	res, err := a.Services.Recon.InvestigateBreak(r.Context(), connect.NewRequest(&reconv1.InvestigateBreakRequest{
		BreakId: r.PathValue("break"), Note: req.Note,
	}))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, breakOut(res.Msg.Break))
}

// ---- shared helpers ----

// decode reads a JSON body strictly. An unknown field is refused, so a client cannot send an actor or a role.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_argument", "the request body is not valid for this route"))
		return false
	}
	return true
}

func uintQuery(r *http.Request, name string, def, max uint64) (uint64, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil || n > max {
		return 0, errors.New(name + " is out of range")
	}
	return n, nil
}

var httpStatus = map[connect.Code]int{
	connect.CodeInvalidArgument:    http.StatusBadRequest,
	connect.CodeNotFound:           http.StatusNotFound,
	connect.CodeAlreadyExists:      http.StatusConflict,
	connect.CodeFailedPrecondition: http.StatusConflict,
	connect.CodePermissionDenied:   http.StatusForbidden,
	connect.CodeUnauthenticated:    http.StatusUnauthorized,
	connect.CodeUnavailable:        http.StatusServiceUnavailable,
}

func writeErr(w http.ResponseWriter, err error) {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		writeJSON(w, http.StatusInternalServerError, errBody("internal", "internal error"))
		return
	}
	status, ok := httpStatus[ce.Code()]
	if !ok {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, errBody(strings.ReplaceAll(ce.Code().String(), "-", "_"), ce.Message()))
}

func errBody(code, msg string) map[string]any {
	return map[string]any{"error": map[string]string{"code": code, "message": msg}}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
