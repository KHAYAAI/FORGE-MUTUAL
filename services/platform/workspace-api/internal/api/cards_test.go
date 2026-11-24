package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	cardsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/cards/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/cards/v1/cardsv1connect"
	lendingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1/lendingv1connect"
	offersv1 "github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1/offersv1connect"
)

type fakeCards struct {
	cardsv1connect.CardServiceClient
	issued *cardsv1.IssueCardRequest
	status *cardsv1.SetCardStatusRequest
	got    *cardsv1.GetCardRequest
}

func (f *fakeCards) IssueCard(_ context.Context, r *connect.Request[cardsv1.IssueCardRequest]) (*connect.Response[cardsv1.IssueCardResponse], error) {
	f.issued = r.Msg
	return connect.NewResponse(&cardsv1.IssueCardResponse{Card: &cardsv1.Card{CardId: "card-1", Last4: "4242", Status: "ACTIVE"}}), nil
}

func (f *fakeCards) SetCardStatus(_ context.Context, r *connect.Request[cardsv1.SetCardStatusRequest]) (*connect.Response[cardsv1.SetCardStatusResponse], error) {
	f.status = r.Msg
	return connect.NewResponse(&cardsv1.SetCardStatusResponse{Card: &cardsv1.Card{CardId: "card-1", Status: "FROZEN"}}), nil
}

func (f *fakeCards) GetCard(_ context.Context, r *connect.Request[cardsv1.GetCardRequest]) (*connect.Response[cardsv1.GetCardResponse], error) {
	f.got = r.Msg
	return connect.NewResponse(&cardsv1.GetCardResponse{Card: &cardsv1.Card{CardId: r.Msg.CardId}}), nil
}

func doCall(h http.Handler, method, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("X-Test-Party", "party-session")
	req.Header.Set("X-Test-Actor", "Thandi")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// With no card or offers service configured, both products say they are not offered: they are off.
func TestCardsAndOffersAreOffUnlessConfigured(t *testing.T) {
	h := (&API{Auth: fakeAuth{}}).Handler()
	for _, path := range []string{"/v1/cards", "/v1/offers", "/v1/referrals"} {
		if w := doCall(h, "GET", path, "", nil); w.Code != http.StatusNotFound {
			t.Fatalf("%s without the service: %d", path, w.Code)
		}
	}
}

// The party is always the session's, never the request's; issuing needs a key; the actor is the session's.
func TestCardsUseTheSessionPartyAndAKey(t *testing.T) {
	f := &fakeCards{}
	h := (&API{Auth: fakeAuth{}, Services: Services{Cards: f}}).Handler()
	body := `{"account_id":"acc-1","label":"Subscriptions","party_id":"someone-else"}`
	if w := doCall(h, "POST", "/v1/cards", body, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("no idempotency key: %d", w.Code)
	}
	w := doCall(h, "POST", "/v1/cards", `{"account_id":"acc-1","label":"Subscriptions"}`, map[string]string{"Idempotency-Key": "c-1"})
	if w.Code != http.StatusCreated || f.issued.PartyId != "party-session" || f.issued.IdempotencyKey != "c-1" || f.issued.AccountId != "acc-1" {
		t.Fatalf("issue: %d %s sent %+v", w.Code, w.Body, f.issued)
	}
	if w := doCall(h, "POST", "/v1/cards", body, map[string]string{"Idempotency-Key": "c-2"}); w.Code != http.StatusBadRequest || f.issued.IdempotencyKey != "c-1" {
		t.Fatalf("a party_id in the body must be refused, and nothing issued: %d %+v", w.Code, f.issued)
	}
	if w := doCall(h, "POST", "/v1/cards/card-1/status", `{"status":"FROZEN"}`, nil); w.Code != http.StatusOK || f.status.PartyId != "party-session" || f.status.Actor != "Thandi" || f.status.CardId != "card-1" {
		t.Fatalf("freeze: %d %+v", w.Code, f.status)
	}
	if w := doCall(h, "GET", "/v1/cards/card-9", "", nil); w.Code != http.StatusOK || f.got.PartyId != "party-session" {
		t.Fatalf("get: %d %+v", w.Code, f.got)
	}
}

type fakeOffers struct {
	offersv1connect.PartnerOffersServiceClient
	referral *offersv1.CreateReferralRequest
	listed   *offersv1.ListOffersRequest
}

const statement = "THIRD-PARTY PRODUCT: provided, licensed and monitored by Acme Wealth, not by Forge Mutual."

func (f *fakeOffers) ListOffers(_ context.Context, r *connect.Request[offersv1.ListOffersRequest]) (*connect.Response[offersv1.ListOffersResponse], error) {
	f.listed = r.Msg
	return connect.NewResponse(&offersv1.ListOffersResponse{Offers: []*offersv1.Offer{{OfferId: "o1", Title: "Acme Savings", ProviderName: "Acme Wealth", Statement: statement, StatementHash: "h1"}}}), nil
}

func (f *fakeOffers) CreateReferral(_ context.Context, r *connect.Request[offersv1.CreateReferralRequest]) (*connect.Response[offersv1.CreateReferralResponse], error) {
	f.referral = r.Msg
	return connect.NewResponse(&offersv1.CreateReferralResponse{Referral: &offersv1.Referral{ReferralId: "ref-1", HandoffUrl: "https://acme.example/go?ref=ref-1"}}), nil
}

// The third-party statement reaches the customer exactly as the offers service wrote it; a referral carries the session party, the key,
// the acknowledgement and the hash the customer saw.
func TestOffersCarryTheStatementUntouchedAndReferralsNeedAnAcknowledgement(t *testing.T) {
	f := &fakeOffers{}
	h := (&API{Auth: fakeAuth{}, Services: Services{Offers: f}}).Handler()
	w := doCall(h, "GET", "/v1/offers?category=INVESTMENTS", "", nil)
	if w.Code != http.StatusOK || f.listed.PartyId != "party-session" || f.listed.Category != "INVESTMENTS" {
		t.Fatalf("list: %d %+v", w.Code, f.listed)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(statement)) || !bytes.Contains(w.Body.Bytes(), []byte(`"statement_hash":"h1"`)) {
		t.Fatalf("the statement must reach the customer unchanged: %s", w.Body)
	}
	if w := doCall(h, "POST", "/v1/offers/o1/referrals", `{"acknowledged_third_party":true,"statement_hash":"h1"}`, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("no idempotency key: %d", w.Code)
	}
	w = doCall(h, "POST", "/v1/offers/o1/referrals", `{"acknowledged_third_party":true,"statement_hash":"h1"}`, map[string]string{"Idempotency-Key": "r-1"})
	if w.Code != http.StatusCreated || f.referral.PartyId != "party-session" || f.referral.OfferId != "o1" || !f.referral.AcknowledgedThirdParty || f.referral.StatementHash != "h1" || f.referral.IdempotencyKey != "r-1" {
		t.Fatalf("referral: %d %s sent %+v", w.Code, w.Body, f.referral)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("https://acme.example/go?ref=ref-1")) {
		t.Fatalf("the provider's address: %s", w.Body)
	}
}

type fakeLendingPA struct {
	lendingv1connect.LendingServiceClient
	accepted *lendingv1.AcceptOfferRequest
}

func (f *fakeLendingPA) GetPreAgreement(_ context.Context, r *connect.Request[lendingv1.GetPreAgreementRequest]) (*connect.Response[lendingv1.GetPreAgreementResponse], error) {
	return connect.NewResponse(&lendingv1.GetPreAgreementResponse{Statement: "PRE-AGREEMENT for " + r.Msg.PartyId, StatementHash: "pa1"}), nil
}

func (f *fakeLendingPA) AcceptOffer(_ context.Context, r *connect.Request[lendingv1.AcceptOfferRequest]) (*connect.Response[lendingv1.AcceptOfferResponse], error) {
	f.accepted = r.Msg
	return connect.NewResponse(&lendingv1.AcceptOfferResponse{Facility: &lendingv1.Facility{FacilityId: "f1"}}), nil
}

func TestThePreAgreementIsShownForTheSessionPartyAndItsHashIsSentOnAcceptance(t *testing.T) {
	f := &fakeLendingPA{}
	h := (&API{Auth: fakeAuth{}, Services: Services{Lending: f}}).Handler()
	if w := doCall(h, "GET", "/v1/facilities/f1/pre-agreement", "", nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("PRE-AGREEMENT for party-session")) {
		t.Fatalf("pre-agreement: %d %s", w.Code, w.Body)
	}
	if w := doCall(h, "POST", "/v1/facilities/f1/accept", `{"accepts_terms":true,"pre_agreement_hash":"pa1"}`, nil); w.Code != http.StatusOK || f.accepted.PreAgreementHash != "pa1" || f.accepted.PartyId != "party-session" {
		t.Fatalf("accept: %d %+v", w.Code, f.accepted)
	}
}
