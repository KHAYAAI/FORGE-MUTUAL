package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	casesv1 "github.com/khayaai/forge-mutual/pkg/api/forge/cases/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/cases/v1/casesv1connect"
	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	reconv1 "github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1/reconv1connect"
)

// The tests use fake downstream clients that record what they were asked. Only the methods a test uses are implemented.

const (
	caseID    = "11111111-1111-4111-8111-111111111111"
	paymentID = "22222222-2222-4222-8222-222222222222"
	breakID   = "33333333-3333-4333-8333-333333333333"
)

type fakeAuth struct{}

// Session reads the test headers: X-Test-Name, and X-Test-Roles as a comma list.
func (fakeAuth) Session(r *http.Request) (Session, error) {
	name := r.Header.Get("X-Test-Name")
	if name == "" {
		return Session{}, errors.New("no session")
	}
	roles := map[string]bool{}
	for _, role := range strings.Split(r.Header.Get("X-Test-Roles"), ",") {
		if role = strings.TrimSpace(role); role != "" {
			roles[role] = true
		}
	}
	return Session{Actor: name, Roles: roles}, nil
}

type fakeCases struct {
	casesv1connect.CasesServiceClient
	listed   *casesv1.ListCasesRequest
	closed   *casesv1.CloseCaseRequest
	closeErr error
}

func (f *fakeCases) ListCases(_ context.Context, req *connect.Request[casesv1.ListCasesRequest]) (*connect.Response[casesv1.ListCasesResponse], error) {
	f.listed = req.Msg
	return connect.NewResponse(&casesv1.ListCasesResponse{Cases: []*casesv1.Case{{CaseId: caseID, Status: "OPEN", Overdue: true}}}), nil
}

func (f *fakeCases) CloseCase(_ context.Context, req *connect.Request[casesv1.CloseCaseRequest]) (*connect.Response[casesv1.CloseCaseResponse], error) {
	f.closed = req.Msg
	if f.closeErr != nil {
		return nil, f.closeErr
	}
	return connect.NewResponse(&casesv1.CloseCaseResponse{Case: &casesv1.Case{CaseId: caseID, Status: "CLOSED", ClosedBy: req.Msg.Actor}}), nil
}

type fakePayments struct {
	paymentsv1connect.PaymentsServiceClient
	approved   *paymentsv1.ApprovePaymentRequest
	repaired   *paymentsv1.ResolveRepairRequest
	approveErr error
}

func (f *fakePayments) ApprovePayment(_ context.Context, req *connect.Request[paymentsv1.ApprovePaymentRequest]) (*connect.Response[paymentsv1.ApprovePaymentResponse], error) {
	f.approved = req.Msg
	if f.approveErr != nil {
		return nil, f.approveErr
	}
	return connect.NewResponse(&paymentsv1.ApprovePaymentResponse{}), nil
}

func (f *fakePayments) ResolveRepair(_ context.Context, req *connect.Request[paymentsv1.ResolveRepairRequest]) (*connect.Response[paymentsv1.ResolveRepairResponse], error) {
	f.repaired = req.Msg
	return connect.NewResponse(&paymentsv1.ResolveRepairResponse{}), nil
}

type fakeRecon struct {
	reconv1connect.ReconServiceClient
	listed       *reconv1.ListBreaksRequest
	investigated *reconv1.InvestigateBreakRequest
}

func (f *fakeRecon) GetGate(_ context.Context, _ *connect.Request[reconv1.GetGateRequest]) (*connect.Response[reconv1.GetGateResponse], error) {
	return connect.NewResponse(&reconv1.GetGateResponse{BreakActive: true, Reasons: []string{"break open: cash, ZAR"}, AsOf: "2026-11-01T08:00:00Z"}), nil
}

func (f *fakeRecon) ListBreaks(_ context.Context, req *connect.Request[reconv1.ListBreaksRequest]) (*connect.Response[reconv1.ListBreaksResponse], error) {
	f.listed = req.Msg
	return connect.NewResponse(&reconv1.ListBreaksResponse{Breaks: []*reconv1.Break{{BreakId: breakID, Status: "OPEN"}}}), nil
}

func (f *fakeRecon) InvestigateBreak(_ context.Context, req *connect.Request[reconv1.InvestigateBreakRequest]) (*connect.Response[reconv1.InvestigateBreakResponse], error) {
	f.investigated = req.Msg
	return connect.NewResponse(&reconv1.InvestigateBreakResponse{Break: &reconv1.Break{BreakId: req.Msg.BreakId, InvestigatedBy: "x"}}), nil
}

type rig struct {
	h        http.Handler
	cases    *fakeCases
	payments *fakePayments
	recon    *fakeRecon
}

func newRig() *rig {
	c := &fakeCases{}
	p := &fakePayments{}
	rc := &fakeRecon{}
	a := &API{Auth: fakeAuth{}, Services: Services{Cases: c, Payments: p, Recon: rc}}
	return &rig{h: a.Handler(), cases: c, payments: p, recon: rc}
}

func (r *rig) do(t *testing.T, method, path, name, roles string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if name != "" {
		req.Header.Set("X-Test-Name", name)
	}
	if roles != "" {
		req.Header.Set("X-Test-Roles", roles)
	}
	rec := httptest.NewRecorder()
	r.h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var out map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %s", rec.Body.String())
	}
	return out["error"]["code"]
}

const ops = RoleOperations
const comp = RoleCompliance

func TestNoSessionIsRefusedBeforeAnyCall(t *testing.T) {
	r := newRig()
	rec := r.do(t, "GET", "/v1/cases", "", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", rec.Code)
	}
	if r.cases.listed != nil {
		t.Fatal("a request without a session must not reach a service")
	}
}

// Closing a case is a compliance decision. An operations person is refused before the case service is asked.
func TestOperationsCannotCloseACase(t *testing.T) {
	r := newRig()
	rec := r.do(t, "POST", "/v1/cases/"+caseID+"/close", "Sipho", ops, map[string]any{"note": "a note of more than twenty characters"})
	if rec.Code != http.StatusForbidden || errorCode(t, rec) != "permission_denied" {
		t.Fatalf("status %d code %s, want 403 permission_denied", rec.Code, errorCode(t, rec))
	}
	if r.cases.closed != nil {
		t.Fatal("the case service must not be asked when the role is missing")
	}
}

// Deciding a payment is an operations decision. A compliance person is refused.
func TestComplianceCannotDecidePayments(t *testing.T) {
	r := newRig()
	rec := r.do(t, "POST", "/v1/payments/"+paymentID+"/approval", "Naledi", comp, map[string]any{"approve": true})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if r.payments.approved != nil {
		t.Fatal("the payments service must not be asked when the role is missing")
	}
}

// The closer is the signed-in person. A body that names someone else is refused, not used.
func TestCloseUsesTheSessionNotTheBody(t *testing.T) {
	r := newRig()
	rec := r.do(t, "POST", "/v1/cases/"+caseID+"/close", "Naledi", comp, map[string]any{
		"note": "documented payroll run, reviewed against the bank statement", "actor": "someone-else",
	})
	if rec.Code != http.StatusBadRequest || r.cases.closed != nil {
		t.Fatalf("a body with an actor: status %d; want 400 and no call", rec.Code)
	}

	rec = r.do(t, "POST", "/v1/cases/"+caseID+"/close", "Naledi", comp, map[string]any{
		"note": "documented payroll run, reviewed against the bank statement",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if r.cases.closed.Actor != "Naledi" || r.cases.closed.CaseId != caseID {
		t.Fatalf("forwarded = %+v", r.cases.closed)
	}
}

// The case service's maker-checker refusal reaches the client with its message.
func TestOpenerCloseIsRefusedWithItsReason(t *testing.T) {
	r := newRig()
	r.cases.closeErr = connect.NewError(connect.CodePermissionDenied, errors.New("the person who opened a case cannot close it"))
	rec := r.do(t, "POST", "/v1/cases/"+caseID+"/close", "system", comp, map[string]any{
		"note": "documented payroll run, reviewed against the bank statement",
	})
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "cannot close it") {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestApprovalNeedsADecisionAndForwardsTheApprover(t *testing.T) {
	r := newRig()
	rec := r.do(t, "POST", "/v1/payments/"+paymentID+"/approval", "Sipho", ops, map[string]any{})
	if rec.Code != http.StatusBadRequest || r.payments.approved != nil {
		t.Fatalf("no decision: status %d; want 400 and no call", rec.Code)
	}

	rec = r.do(t, "POST", "/v1/payments/"+paymentID+"/approval", "Sipho", ops, map[string]any{"approve": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if r.payments.approved.Approver != "Sipho" || r.payments.approved.Approve || r.payments.approved.PaymentId != paymentID {
		t.Fatalf("forwarded = %+v", r.payments.approved)
	}
	if !strings.Contains(rec.Body.String(), `"REFUSED"`) {
		t.Fatalf("response = %s", rec.Body.String())
	}
}

func TestDownstreamStateRefusalIsAConflict(t *testing.T) {
	r := newRig()
	r.payments.approveErr = connect.NewError(connect.CodeFailedPrecondition, errors.New("the payment is not awaiting approval"))
	rec := r.do(t, "POST", "/v1/payments/"+paymentID+"/approval", "Sipho", ops, map[string]any{"approve": true})
	if rec.Code != http.StatusConflict || errorCode(t, rec) != "failed_precondition" {
		t.Fatalf("status %d code %s, want 409", rec.Code, errorCode(t, rec))
	}
}

func TestRepairForwardsTheOperator(t *testing.T) {
	r := newRig()
	rec := r.do(t, "POST", "/v1/payments/"+paymentID+"/repair", "Sipho", ops, map[string]any{"settled": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if r.payments.repaired.Operator != "Sipho" || !r.payments.repaired.Settled {
		t.Fatalf("forwarded = %+v", r.payments.repaired)
	}
}

func TestInvestigateForwardsTheNote(t *testing.T) {
	r := newRig()
	rec := r.do(t, "POST", "/v1/recon/breaks/"+breakID+"/investigate", "Sipho", ops, map[string]any{
		"note": "the feed was re-sent after a timeout; the duplicate was reversed",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if r.recon.investigated.BreakId != breakID || !strings.HasPrefix(r.recon.investigated.Note, "the feed") {
		t.Fatalf("forwarded = %+v", r.recon.investigated)
	}
}

func TestCaseListDefaultsToOpenAndBoundsTheLimit(t *testing.T) {
	r := newRig()
	rec := r.do(t, "GET", "/v1/cases", "Naledi", comp, nil)
	if rec.Code != http.StatusOK || r.cases.listed == nil || !r.cases.listed.OpenOnly {
		t.Fatalf("default list: status %d, request %+v", rec.Code, r.cases.listed)
	}
	if !strings.Contains(rec.Body.String(), `"overdue":true`) {
		t.Fatalf("overdue is not passed through: %s", rec.Body.String())
	}

	rec = r.do(t, "GET", "/v1/cases?status=all", "Naledi", comp, nil)
	if rec.Code != http.StatusOK || r.cases.listed.OpenOnly {
		t.Fatalf("status=all must include closed cases: %+v", r.cases.listed)
	}
	for _, q := range []string{"?status=closed", "?limit=0", "?limit=501"} {
		if rec := r.do(t, "GET", "/v1/cases"+q, "Naledi", comp, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", q, rec.Code)
		}
	}
}

func TestGateReportsItsReasons(t *testing.T) {
	r := newRig()
	rec := r.do(t, "GET", "/v1/recon/gate", "Sipho", ops, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["break_active"] != true || len(out["reasons"].([]any)) != 1 {
		t.Fatalf("gate = %v", out)
	}
}

func TestBreaksDefaultToOpen(t *testing.T) {
	r := newRig()
	r.do(t, "GET", "/v1/recon/breaks", "Sipho", ops, nil)
	if r.recon.listed == nil || !r.recon.listed.OpenOnly {
		t.Fatalf("default must be open breaks: %+v", r.recon.listed)
	}
	r.do(t, "GET", "/v1/recon/breaks?open_only=false", "Sipho", ops, nil)
	if r.recon.listed.OpenOnly {
		t.Fatal("open_only=false must list all breaks")
	}
}
