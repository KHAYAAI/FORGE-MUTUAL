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
	fee        *offersv1.RecordReferralFeeRequest
}

func (f *fakePartners) ListAdminReferrals(context.Context, *connect.Request[offersv1.ListAdminReferralsRequest]) (*connect.Response[offersv1.ListAdminReferralsResponse], error) {
	return connect.NewResponse(&offersv1.ListAdminReferralsResponse{Referrals: []*offersv1.AdminReferral{{ReferralId: "r1", DeliveryStatus: "SENT"}}}), nil
}

func (f *fakePartners) RecordReferralFee(_ context.Context, r *connect.Request[offersv1.RecordReferralFeeRequest]) (*connect.Response[offersv1.RecordReferralFeeResponse], error) {
	f.fee = r.Msg
	return connect.NewResponse(&offersv1.RecordReferralFeeResponse{AwaitingSecondPerson: true}), nil
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
	reg := `{"provider_id":"acme","name":"Acme Wealth","licence_type":"FSP","regulator":"FSCA","licence_number":"FSP 1","licence_source_url":"https://www.fsca.co.za/x","delivery_url":"https://hooks.acme.example/forge"}`
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/v1/partners/providers", ""}, {"GET", "/v1/partners/offers", ""}, {"POST", "/v1/partners/providers", reg},
		{"POST", "/v1/partners/providers/acme/verify", `{}`}, {"POST", "/v1/partners/providers/acme/suspend", `{"reason":"x"}`},
		{"POST", "/v1/partners/offers/o1/status", `{"status":"ACTIVE"}`},
	} {
		if w := partnerCall(h, c.method, c.path, c.body, "operations,credit"); w.Code != http.StatusForbidden {
			t.Fatalf("%s %s without the compliance role: %d", c.method, c.path, w.Code)
		}
	}
	if w := partnerCall(h, "POST", "/v1/partners/providers", reg, "compliance"); w.Code != http.StatusCreated || f.registered.Actor != "Anna (compliance)" || f.registered.LicenceNumber != "FSP 1" || f.registered.DeliveryUrl != "https://hooks.acme.example/forge" {
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
	w := partnerCall(h, "POST", "/v1/partners/offers", `{"offer_id":"o1","provider_id":"acme","category":"INVESTMENTS","title":"T","summary":"S","handoff_url":"https://acme.example/go","referral_fee_cap_minor":"150000","fee_disclosure":"Paid out of the provider's own fee."}`, "compliance")
	if w.Code != http.StatusOK || f.upserted.Actor != "Anna (compliance)" || f.upserted.ReferralFeeCapMinor != "150000" || !bytes.Contains(w.Body.Bytes(), []byte("provided, licensed and monitored by Acme")) {
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

// Compliance and operations may look at referrals; only operations records a fee, as themselves, with no approver of their choosing.
func TestReferralsAreVisibleToComplianceAndOperationsAndFeesAreOperationsAsTheSessionActor(t *testing.T) {
	f := &fakePartners{}
	h := (&API{Auth: fakeAuth{}, Services: Services{Partners: f}}).Handler()
	for _, roles := range []string{"compliance", "operations"} {
		if w := partnerCall(h, "GET", "/v1/partners/referrals", "", roles); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"delivery_status":"SENT"`)) {
			t.Fatalf("%s should see referrals: %d %s", roles, w.Code, w.Body)
		}
	}
	if w := partnerCall(h, "GET", "/v1/partners/referrals", "", "credit"); w.Code != http.StatusForbidden {
		t.Fatalf("credit must not: %d", w.Code)
	}
	body := `{"amount_minor":"30000","bank_reference":"STMT-1/001"}`
	for _, roles := range []string{"compliance", "credit"} {
		if w := partnerCall(h, "POST", "/v1/partners/referrals/r1/fee", body, roles); w.Code != http.StatusForbidden || f.fee != nil {
			t.Fatalf("%s must not record a fee: %d", roles, w.Code)
		}
	}
	if w := partnerCall(h, "POST", "/v1/partners/referrals/r1/fee", `{"amount_minor":"1","bank_reference":"STMT-1/002","officer":"someone else","approver":"x"}`, "operations"); w.Code != http.StatusBadRequest {
		t.Fatalf("an officer or approver in the body must be refused: %d", w.Code)
	}
	w := partnerCall(h, "POST", "/v1/partners/referrals/r1/fee", body, "operations")
	if w.Code != http.StatusOK || f.fee.ReferralId != "r1" || f.fee.Officer != "Anna (compliance)" || f.fee.AmountMinor != "30000" || !bytes.Contains(w.Body.Bytes(), []byte(`"awaiting_second_person":true`)) {
		t.Fatalf("fee: %d %s %+v", w.Code, w.Body, f.fee)
	}
}
