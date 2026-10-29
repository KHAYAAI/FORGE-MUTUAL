package service

import (
	"context"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	consentv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1"
	"github.com/khayaai/forge-mutual/services/platform/consent-service/internal/store"
)

// These tests need PostgreSQL: set CONSENT_TEST_DSN to a throwaway database; tables are truncated.

const party = "11111111-1111-4111-8111-111111111111"

func newService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	dsn := os.Getenv("CONSENT_TEST_DSN")
	if dsn == "" {
		t.Skip("CONSENT_TEST_DSN not set")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if _, err := st.Pool().Exec(ctx, `truncate consent, idempotency, outbox`); err != nil {
		t.Fatal(err)
	}
	return New(st), st
}

func grant(key string, purpose string, scopes ...string) *connect.Request[consentv1.GrantRequest] {
	return connect.NewRequest(&consentv1.GrantRequest{
		IdempotencyKey: key, PartyId: party, Purpose: purpose, Scopes: scopes,
		DataSources: []string{"bank_statements"}, EvidenceRef: "signed-form-42", Actor: "onboarding",
	})
}

func eventCount(t *testing.T, st *store.Store, topic string) int {
	t.Helper()
	var n int
	if err := st.Pool().QueryRow(context.Background(), `select count(*) from outbox where topic = $1`, topic).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func check(t *testing.T, s *Service, purpose, scope string) *consentv1.CheckResponse {
	t.Helper()
	res, err := s.Check(context.Background(), connect.NewRequest(&consentv1.CheckRequest{PartyId: party, Purpose: purpose, Scope: scope}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg
}

func TestGrantedConsentAllowsOnlyItsPurposeAndScope(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.Grant(ctx, grant("g1", "kyb.processing", "company.registry", "company.directors")); err != nil {
		t.Fatal(err)
	}
	if r := check(t, s, "kyb.processing", "company.registry"); !r.Allowed || r.ConsentId == "" {
		t.Fatalf("granted scope must be allowed: %+v", r)
	}
	if r := check(t, s, "kyb.processing", "bank.balances"); r.Allowed || r.Reason != "no_valid_consent" {
		t.Fatalf("ungranted scope must be denied: %+v", r)
	}
	if r := check(t, s, "marketing.email", "company.registry"); r.Allowed {
		t.Fatal("a different purpose must be denied")
	}
}

func TestRevocationTakesEffectImmediately(t *testing.T) {
	s, st := newService(t)
	ctx := context.Background()
	g, err := s.Grant(ctx, grant("g1", "kyb.processing", "company.registry"))
	if err != nil {
		t.Fatal(err)
	}
	if !check(t, s, "kyb.processing", "company.registry").Allowed {
		t.Fatal("precondition: allowed before revocation")
	}
	if _, err := s.Revoke(ctx, connect.NewRequest(&consentv1.RevokeRequest{ConsentId: g.Msg.Consent.ConsentId, Actor: "party-owner"})); err != nil {
		t.Fatal(err)
	}
	if check(t, s, "kyb.processing", "company.registry").Allowed {
		t.Fatal("revoked consent must deny on the next check")
	}
	// Revoking again changes nothing and writes no second event.
	if _, err := s.Revoke(ctx, connect.NewRequest(&consentv1.RevokeRequest{ConsentId: g.Msg.Consent.ConsentId, Actor: "party-owner"})); err != nil {
		t.Fatal(err)
	}
	if n := eventCount(t, st, TopicRevoked); n != 1 {
		t.Fatalf("revoked events = %d, want 1", n)
	}
	if n := eventCount(t, st, TopicGranted); n != 1 {
		t.Fatalf("granted events = %d, want 1", n)
	}
}

func TestExpiredConsentDenies(t *testing.T) {
	s, st := newService(t)
	ctx := context.Background()
	// Expiry in the past cannot be granted through the API, so the row is written directly. This
	// tests the check, not the grant validation.
	past := time.Now().Add(-time.Hour).UTC()
	if err := store.Insert(ctx, st.Pool(), store.Consent{
		ID: "22222222-2222-4222-8222-222222222222", PartyID: party, Purpose: "kyb.processing",
		Scopes: []string{"company.registry"}, DataSources: []string{"x"}, GrantedAt: past.Add(-time.Hour),
		ExpiresAt: &past, EvidenceRef: "old", GrantedBy: "test",
	}); err != nil {
		t.Fatal(err)
	}
	if check(t, s, "kyb.processing", "company.registry").Allowed {
		t.Fatal("expired consent must deny")
	}
}

func TestGrantIsIdempotent(t *testing.T) {
	s, st := newService(t)
	ctx := context.Background()
	a, err := s.Grant(ctx, grant("g1", "kyb.processing", "company.registry"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Grant(ctx, grant("g1", "kyb.processing", "company.registry"))
	if err != nil || a.Msg.Consent.ConsentId != b.Msg.Consent.ConsentId {
		t.Fatalf("replay must return the same consent: %v", err)
	}
	if n := eventCount(t, st, TopicGranted); n != 1 {
		t.Fatalf("granted events = %d, want 1", n)
	}
	if _, err := s.Grant(ctx, grant("g1", "kyb.processing", "company.directors")); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("same key, different request: want AlreadyExists, got %v", err)
	}
}

func TestGrantValidation(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	bad := []*connect.Request[consentv1.GrantRequest]{
		connect.NewRequest(&consentv1.GrantRequest{IdempotencyKey: "x", PartyId: "not-a-uuid", Purpose: "kyb.processing", Scopes: []string{"a.b"}, EvidenceRef: "e", Actor: "a"}),
		grant("x", "Bad Purpose", "company.registry"),
		grant("x", "kyb.processing"),
		grant("x", "kyb.processing", "company.registry", "company.registry"),
		connect.NewRequest(&consentv1.GrantRequest{IdempotencyKey: "x", PartyId: party, Purpose: "kyb.processing", Scopes: []string{"a.b"}, EvidenceRef: "", Actor: "a"}),
	}
	for i, r := range bad {
		if _, err := s.Grant(ctx, r); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("case %d: want InvalidArgument, got %v", i, err)
		}
	}
	past := connect.NewRequest(&consentv1.GrantRequest{IdempotencyKey: "p", PartyId: party, Purpose: "kyb.processing",
		Scopes: []string{"company.registry"}, DataSources: []string{"x"}, EvidenceRef: "e", Actor: "a",
		ExpiresAt: timestamppb.New(time.Now().Add(-time.Minute))})
	if _, err := s.Grant(ctx, past); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expiry in the past must be refused: %v", err)
	}
}

func TestListShowsRevokedToo(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	g, _ := s.Grant(ctx, grant("g1", "kyb.processing", "company.registry"))
	_, _ = s.Revoke(ctx, connect.NewRequest(&consentv1.RevokeRequest{ConsentId: g.Msg.Consent.ConsentId, Actor: "owner"}))
	res, err := s.List(ctx, connect.NewRequest(&consentv1.ListRequest{PartyId: party}))
	if err != nil || len(res.Msg.Consents) != 1 || res.Msg.Consents[0].RevokedAt == nil {
		t.Fatalf("list = %v, %v", res, err)
	}
}
