package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	consentv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1/consentv1connect"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
)

type fakeParty struct {
	partyv1connect.PartyServiceClient
	asked string
}

func (f *fakeParty) GetParty(_ context.Context, r *connect.Request[partyv1.GetPartyRequest]) (*connect.Response[partyv1.GetPartyResponse], error) {
	f.asked = r.Msg.PartyId
	return connect.NewResponse(&partyv1.GetPartyResponse{Party: &partyv1.Party{PartyId: r.Msg.PartyId, LegalName: "Thandi Dube",
		NationalIdMasked: "*********9087", CreatedAt: timestamppb.Now()}}), nil
}

type fakeConsent struct {
	consentv1connect.ConsentServiceClient
	asked string
}

func (f *fakeConsent) List(_ context.Context, r *connect.Request[consentv1.ListRequest]) (*connect.Response[consentv1.ListResponse], error) {
	f.asked = r.Msg.PartyId
	return connect.NewResponse(&consentv1.ListResponse{Consents: []*consentv1.Consent{{Purpose: "kyc.processing", GrantedAt: timestamppb.Now()}}}), nil
}

// A customer downloads what Forge holds about them: only their own, with the identity number masked.
func TestMyDataIsTheSessionPartysOwnAndMasked(t *testing.T) {
	p, c := &fakeParty{}, &fakeConsent{}
	h := (&API{Auth: fakeAuth{}, Services: Services{Party: p, Consent: c, Consolidated: &fakeConsolidated{}}}).Handler()
	w := asCustomer(t, h, "GET", "/v1/my-data", nil)
	if w.Code != http.StatusOK || p.asked != "party-session" || c.asked != "party-session" {
		t.Fatalf("my data: %d %s; asked %q %q", w.Code, w.Body, p.asked, c.asked)
	}
	body := w.Body.String()
	if !strings.Contains(body, "*********9087") || !strings.Contains(body, "kyc.processing") || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("body %s", body)
	}
}

type frozenParty struct{ fakeParty }

func (f *frozenParty) GetParty(_ context.Context, r *connect.Request[partyv1.GetPartyRequest]) (*connect.Response[partyv1.GetPartyResponse], error) {
	return connect.NewResponse(&partyv1.GetPartyResponse{Party: &partyv1.Party{PartyId: r.Msg.PartyId, Status: partyv1.PartyStatus_PARTY_STATUS_FROZEN}}), nil
}

// A frozen party may read but not act, and is told nothing about why.
func TestAFrozenPartyMayReadButNotAct(t *testing.T) {
	cons := &fakeConsolidated{}
	h := (&API{Auth: fakeAuth{}, Services: Services{Party: &frozenParty{}, Consolidated: cons, Onboarding: &fakeCurrency{}}}).Handler()
	if w := asCustomer(t, h, "GET", "/v1/accounts", nil); w.Code != http.StatusOK {
		t.Fatalf("reading accounts while frozen: %d", w.Code)
	}
	w := asCustomer(t, h, "POST", "/v1/accounts", map[string]string{"asset": "EUR"})
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "account_restricted") {
		t.Fatalf("acting while frozen: %d %s", w.Code, w.Body)
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "sanction") || strings.Contains(strings.ToLower(w.Body.String()), "frozen") {
		t.Fatalf("the refusal discloses the reason: %s", w.Body)
	}
}
