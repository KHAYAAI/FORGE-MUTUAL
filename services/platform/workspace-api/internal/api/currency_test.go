package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
)

type fakeCurrency struct {
	fakeOnboarding
	paymentsv1connect.CrossBorderServiceClient
	paymentsv1connect.InboundReferenceServiceClient
	opened     *onboardingv1.OpenCurrencyAccountRequest
	classified *paymentsv1.ClassifyCrossBorderCreditRequest
	listedFor  string
}

func (f *fakeCurrency) OpenCurrencyAccount(_ context.Context, r *connect.Request[onboardingv1.OpenCurrencyAccountRequest]) (*connect.Response[onboardingv1.OpenCurrencyAccountResponse], error) {
	f.opened = r.Msg
	return connect.NewResponse(&onboardingv1.OpenCurrencyAccountResponse{AccountId: "acc-eur", Asset: "EUR", Reference: "FORGEEURPB1", Partner: "p"}), nil
}

func (f *fakeCurrency) ClassifyCrossBorderCredit(_ context.Context, r *connect.Request[paymentsv1.ClassifyCrossBorderCreditRequest]) (*connect.Response[paymentsv1.ClassifyCrossBorderCreditResponse], error) {
	f.classified = r.Msg
	return connect.NewResponse(&paymentsv1.ClassifyCrossBorderCreditResponse{Credit: &paymentsv1.CrossBorderCredit{CreditId: r.Msg.CreditId, Status: "CLASSIFIED"}}), nil
}

func (f *fakeCurrency) ListInboundReferences(_ context.Context, r *connect.Request[paymentsv1.ListInboundReferencesRequest]) (*connect.Response[paymentsv1.ListInboundReferencesResponse], error) {
	f.listedFor = r.Msg.PartyId
	return connect.NewResponse(&paymentsv1.ListInboundReferencesResponse{}), nil
}

func asCustomer(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("X-Test-Party", "party-session")
	req.Header.Set("X-Test-Actor", "Thandi")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// The account, the reason and the receiving details are always the session party's; a body naming a party is refused.
func TestCurrencyRoutesUseTheSessionParty(t *testing.T) {
	f := &fakeCurrency{}
	h := (&API{Auth: fakeAuth{}, Services: Services{Onboarding: f, CrossBorder: f, References: f}}).Handler()

	if w := asCustomer(t, h, "POST", "/v1/accounts", map[string]string{"asset": "EUR"}); w.Code != http.StatusCreated {
		t.Fatalf("open: %d %s", w.Code, w.Body)
	}
	if f.opened.PartyId != "party-session" || f.opened.Actor != "Thandi" || f.opened.Asset != "EUR" {
		t.Fatalf("open sent %+v", f.opened)
	}
	if w := asCustomer(t, h, "POST", "/v1/accounts", map[string]string{"asset": "EUR", "party_id": "someone-else"}); w.Code != http.StatusBadRequest {
		t.Fatalf("a body naming a party: %d", w.Code)
	}
	if w := asCustomer(t, h, "POST", "/v1/cross-border-credits/USD:mv-1/reason", map[string]string{"reason_code": "services_software", "note": "INV-1"}); w.Code != http.StatusOK {
		t.Fatalf("classify: %d %s", w.Code, w.Body)
	}
	if c := f.classified; c.PartyId != "party-session" || c.CreditId != "USD:mv-1" || c.Actor != "Thandi" || c.ReasonCode != "services_software" {
		t.Fatalf("classify sent %+v", c)
	}
	if w := asCustomer(t, h, "GET", "/v1/receiving-details", nil); w.Code != http.StatusOK || f.listedFor != "party-session" {
		t.Fatalf("receiving details: %d for %q", w.Code, f.listedFor)
	}
	req := httptest.NewRequest("POST", "/v1/accounts", bytes.NewBufferString(`{"asset":"EUR"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no session: %d", w.Code)
	}
}
