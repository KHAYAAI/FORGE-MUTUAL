package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	offersv1 "github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1/offersv1connect"
)

type fakePartners struct {
	offersv1connect.PartnerAdminServiceClient
	registered *offersv1.RegisterProviderRequest
	verified   *offersv1.VerifyLicenceRequest
	suspended  *offersv1.SuspendProviderRequest
	upserted   *offersv1.UpsertOfferRequest
	status     *offersv1.SetOfferStatusRequest
}

func (f *fakePartners) RegisterProvider(_ context.Context, r *connect.Request[offersv1.RegisterProviderRequest]) (*connect.Response[offersv1.RegisterProviderResponse], error) {
	f.registered = r.Msg
	return connect.NewResponse(&offersv1.RegisterProviderResponse{Provider: &offersv1.Provider{ProviderId: r.Msg.ProviderId, Status: "PENDING"}}), nil
}

func (f *fakePartners) VerifyLicence(_ context.Context, r *connect.Request[offersv1.VerifyLicenceRequest]) (*connect.Response[offersv1.VerifyLicenceResponse], error) {
	f.verified = r.Msg
	return connect.NewResponse(&offersv1.VerifyLicenceResponse{Provider: &offersv1.Provider{ProviderId: r.Msg.ProviderId, Status: "ACTIVE"}}), nil
}

func (f *fakePartners) SuspendProvider(_ context.Context, r *connect.Request[offersv1.SuspendProviderRequest]) (*connect.Response[offersv1.SuspendProviderResponse], error) {
	f.suspended = r.Msg
	return connect.NewResponse(&offersv1.SuspendProviderResponse{Provider: &offersv1.Provider{ProviderId: r.Msg.ProviderId, Status: "SUSPENDED"}}), nil
}

func (f *fakePartners) UpsertOffer(_ context.Context, r *connect.Request[offersv1.UpsertOfferRequest]) (*connect.Response[offersv1.UpsertOfferResponse], error) {
	f.upserted = r.Msg
	return connect.NewResponse(&offersv1.UpsertOfferResponse{Offer: &offersv1.Offer{OfferId: r.Msg.OfferId, Statement: "THIRD-PARTY PRODUCT: provided, licensed and monitored by Acme, not by Forge Mutual."}}), nil
}

func (f *fakePartners) SetOfferStatus(_ context.Context, r *connect.Request[offersv1.SetOfferStatusRequest]) (*connect.Response[offersv1.SetOfferStatusResponse], error) {
	f.status = r.Msg
	return connect.NewResponse(&offersv1.SetOfferStatusResponse{Status: r.Msg.Status}), nil
}

func partnerCall(h http.Handler, method, path, body, roles string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("X-Test-Name", "Anna (compliance)")
	req.Header.Set("X-Test-Roles", roles)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// Only compliance may touch third-party providers, the actor is always the session's, and nothing is offered without the service.
func TestPartnerAdministrationIsForComplianceAndUsesTheSessionActor(t *testing.T) {
	f := &fakePartners{}
	h := (&API{Auth: fakeAuth{}, Services: Services{Partners: f}}).Handler()
	reg := `{"provider_id":"acme","name":"Acme Wealth","licence_type":"FSP","regulator":"FSCA","licence_number":"FSP 1","licence_source_url":"https://www.fsca.co.za/x"}`
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/v1/partners/providers", ""}, {"GET", "/v1/partners/offers", ""}, {"POST", "/v1/partners/providers", reg},
		{"POST", "/v1/partners/providers/acme/verify", `{}`}, {"POST", "/v1/partners/providers/acme/suspend", `{"reason":"x"}`},
		{"POST", "/v1/partners/offers/o1/status", `{"status":"ACTIVE"}`},
	} {
		if w := partnerCall(h, c.method, c.path, c.body, "operations,credit"); w.Code != http.StatusForbidden {
			t.Fatalf("%s %s without the compliance role: %d", c.method, c.path, w.Code)
		}
	}
	if w := partnerCall(h, "POST", "/v1/partners/providers", reg, "compliance"); w.Code != http.StatusCreated || f.registered.Actor != "Anna (compliance)" || f.registered.LicenceNumber != "FSP 1" {
		t.Fatalf("register: %d %s %+v", w.Code, w.Body, f.registered)
	}
	if w := partnerCall(h, "POST", "/v1/partners/providers", `{"provider_id":"x","actor":"someone else"}`, "compliance"); w.Code != http.StatusBadRequest {
		t.Fatalf("an actor in the body must be refused: %d", w.Code)
	}
	if w := partnerCall(h, "POST", "/v1/partners/providers/acme/verify", `{"licence_expires_at":"2028-01-01T00:00:00Z"}`, "compliance"); w.Code != http.StatusOK || f.verified.ProviderId != "acme" || f.verified.Actor != "Anna (compliance)" {
		t.Fatalf("verify: %d %+v", w.Code, f.verified)
	}
	if w := partnerCall(h, "POST", "/v1/partners/providers/acme/suspend", `{"reason":"FSCA notice"}`, "compliance"); w.Code != http.StatusOK || f.suspended.Reason != "FSCA notice" {
		t.Fatalf("suspend: %d %+v", w.Code, f.suspended)
	}
	w := partnerCall(h, "POST", "/v1/partners/offers", `{"offer_id":"o1","provider_id":"acme","category":"INVESTMENTS","title":"T","summary":"S","handoff_url":"https://acme.example/go","fee_disclosure":"None"}`, "compliance")
	if w.Code != http.StatusOK || f.upserted.Actor != "Anna (compliance)" || !bytes.Contains(w.Body.Bytes(), []byte("provided, licensed and monitored by Acme")) {
		t.Fatalf("upsert: %d %s", w.Code, w.Body)
	}
	if w := partnerCall(h, "POST", "/v1/partners/offers/o1/status", `{"status":"ACTIVE"}`, "compliance"); w.Code != http.StatusOK || f.status.OfferId != "o1" || f.status.Actor != "Anna (compliance)" {
		t.Fatalf("status: %d %+v", w.Code, f.status)
	}
	none := (&API{Auth: fakeAuth{}}).Handler()
	if w := partnerCall(none, "GET", "/v1/partners/providers", "", "compliance"); w.Code != http.StatusNotFound {
		t.Fatalf("without the service: %d", w.Code)
	}
}
