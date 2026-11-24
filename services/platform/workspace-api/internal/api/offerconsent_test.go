package api

import (
	"bytes"
	"context"
	"net/http"
	"regexp"
	"testing"

	"connectrpc.com/connect"

	consentv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1/consentv1connect"
	offersv1 "github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1"
)

type fakeShareConsent struct {
	consentv1connect.ConsentServiceClient
	granted *consentv1.GrantRequest
	revoked string
	active  bool
}

func (f *fakeShareConsent) Check(_ context.Context, r *connect.Request[consentv1.CheckRequest]) (*connect.Response[consentv1.CheckResponse], error) {
	ok := f.active && r.Msg.Purpose == "share_with_partner" && r.Msg.Scope == "referral:acme"
	return connect.NewResponse(&consentv1.CheckResponse{Allowed: ok, ConsentId: "consent-1"}), nil
}

func (f *fakeShareConsent) Grant(_ context.Context, r *connect.Request[consentv1.GrantRequest]) (*connect.Response[consentv1.GrantResponse], error) {
	f.granted, f.active = r.Msg, true
	return connect.NewResponse(&consentv1.GrantResponse{}), nil
}

func (f *fakeShareConsent) Revoke(_ context.Context, r *connect.Request[consentv1.RevokeRequest]) (*connect.Response[consentv1.RevokeResponse], error) {
	f.revoked, f.active = r.Msg.ConsentId, false
	return connect.NewResponse(&consentv1.RevokeResponse{}), nil
}

type fakeOffersWithProvider struct{ fakeOffers }

func (f *fakeOffersWithProvider) ListOffers(_ context.Context, _ *connect.Request[offersv1.ListOffersRequest]) (*connect.Response[offersv1.ListOffersResponse], error) {
	return connect.NewResponse(&offersv1.ListOffersResponse{Offers: []*offersv1.Offer{{OfferId: "o1", ProviderId: "acme", ProviderName: "Acme Wealth", Statement: statement, StatementHash: "h1", Shares: []string{"your name"}}}}), nil
}

// The customer can see, give and withdraw consent for a provider they were shown; it needs the current statement and an explicit agreement;
// the party, purpose and scope are right; and a provider that was not shown cannot be consented to.
func TestTheCustomerGivesAndWithdrawsConsentForAProviderTheyWereShown(t *testing.T) {
	c := &fakeShareConsent{}
	h := (&API{Auth: fakeAuth{}, Services: Services{Offers: &fakeOffersWithProvider{}, Consent: c}}).Handler()
	w := doCall(h, "GET", "/v1/offers/o1/consent", "", nil)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"granted":false`)) || !bytes.Contains(w.Body.Bytes(), []byte("your name")) || !bytes.Contains(w.Body.Bytes(), []byte("Acme Wealth")) {
		t.Fatalf("before: %d %s", w.Code, w.Body)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("email")) || bytes.Contains(w.Body.Bytes(), []byte("phone")) {
		t.Fatalf("the customer is told only what the offers service will send, which is their name: %s", w.Body)
	}
	if w := doCall(h, "GET", "/v1/offers/not-shown/consent", "", nil); w.Code != http.StatusNotFound {
		t.Fatalf("an offer the customer was not shown: %d", w.Code)
	}
	if w := doCall(h, "POST", "/v1/offers/o1/consent", `{"agrees":true,"statement_hash":"h1"}`, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("no idempotency key: %d", w.Code)
	}
	key := map[string]string{"Idempotency-Key": "c-1"}
	if w := doCall(h, "POST", "/v1/offers/o1/consent", `{"agrees":false,"statement_hash":"h1"}`, key); w.Code != http.StatusConflict || c.granted != nil {
		t.Fatalf("without agreeing: %d", w.Code)
	}
	if w := doCall(h, "POST", "/v1/offers/o1/consent", `{"agrees":true,"statement_hash":"old"}`, key); w.Code != http.StatusConflict || c.granted != nil {
		t.Fatalf("with a stale statement: %d", w.Code)
	}
	w = doCall(h, "POST", "/v1/offers/o1/consent", `{"agrees":true,"statement_hash":"h1"}`, key)
	if w.Code != http.StatusCreated || c.granted == nil || c.granted.PartyId != "party-session" || c.granted.Purpose != "share_with_partner" ||
		len(c.granted.Scopes) != 1 || c.granted.Scopes[0] != "referral:acme" || c.granted.Actor != "Thandi" || c.granted.IdempotencyKey != "c-1" {
		t.Fatalf("grant: %d %s %+v", w.Code, w.Body, c.granted)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"granted":true`)) || c.granted.EvidenceRef != "offer:o1;statement:h1" {
		t.Fatalf("granted view and evidence: %s %q", w.Body, c.granted.EvidenceRef)
	}
	w = doCall(h, "POST", "/v1/offers/o1/consent/withdraw", "", nil)
	if w.Code != http.StatusOK || c.revoked != "consent-1" || !bytes.Contains(w.Body.Bytes(), []byte(`"granted":false`)) {
		t.Fatalf("withdraw: %d %s revoked %q", w.Code, w.Body, c.revoked)
	}
	if w := doCall(h, "POST", "/v1/offers/o1/consent/withdraw", "", nil); w.Code != http.StatusOK {
		t.Fatalf("withdrawing again is harmless: %d", w.Code)
	}
}

// The consent service accepts only plain purposes and scopes; ours must match its patterns.
func TestTheConsentPurposeAndScopeMatchTheConsentServicesPatterns(t *testing.T) {
	if !regexp.MustCompile(`^[a-z][a-z0-9_.]{2,63}$`).MatchString(partnerConsentPurpose) || !regexp.MustCompile(`^[a-z][a-z0-9_.:-]{1,63}$`).MatchString(partnerConsentScope+"acme-wealth") {
		t.Fatal("the purpose or scope would be refused by the consent service")
	}
}
