package service

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/services/platform/party-service/internal/sealer"
	"github.com/khayaai/forge-mutual/services/platform/party-service/internal/store"
)

// These tests need PostgreSQL. Set PARTY_TEST_DSN to a throwaway database; tables are truncated.

func newService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	dsn := os.Getenv("PARTY_TEST_DSN")
	if dsn == "" {
		t.Skip("PARTY_TEST_DSN not set")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if _, err := st.Pool().Exec(ctx, `truncate relationship, party, idempotency, outbox`); err != nil {
		t.Fatal(err)
	}
	sl, err := sealer.New(bytes.Repeat([]byte{7}, 32), bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return New(st, sl, slog.New(slog.NewTextHandler(io.Discard, nil))), st
}

func person(key, name, nationalID string) *connect.Request[partyv1.CreatePartyRequest] {
	return connect.NewRequest(&partyv1.CreatePartyRequest{
		IdempotencyKey: key, Type: partyv1.PartyType_PARTY_TYPE_PERSON, LegalName: name,
		NationalId: nationalID, Country: "ZA", Actor: "test",
	})
}

func company(key, name, reg string) *connect.Request[partyv1.CreatePartyRequest] {
	return connect.NewRequest(&partyv1.CreatePartyRequest{
		IdempotencyKey: key, Type: partyv1.PartyType_PARTY_TYPE_COMPANY, LegalName: name,
		RegistrationNumber: reg, Country: "ZA", Actor: "test",
	})
}

func mustCreate(t *testing.T, s *Service, r *connect.Request[partyv1.CreatePartyRequest]) *partyv1.Party {
	t.Helper()
	res, err := s.CreateParty(context.Background(), r)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return res.Msg.Party
}

func statusEvents(t *testing.T, st *store.Store) []*eventsv1.PartyStatusChanged {
	t.Helper()
	rows, err := st.Pool().Query(context.Background(), `select payload from outbox order by seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []*eventsv1.PartyStatusChanged
	for rows.Next() {
		var p []byte
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		ev := &eventsv1.PartyStatusChanged{}
		if err := proto.Unmarshal(p, ev); err != nil {
			t.Fatal(err)
		}
		out = append(out, ev)
	}
	return out
}

func codeOf(err error) connect.Code { return connect.CodeOf(err) }

func TestPersonIsSealedAndMasked(t *testing.T) {
	s, st := newService(t)
	p := mustCreate(t, s, person("p1", "Thandi Dube", "8001010012345"))
	if p.NationalIdMasked != "*********2345" || p.Status != partyv1.PartyStatus_PARTY_STATUS_PROSPECT {
		t.Fatalf("party = %+v", p)
	}
	var raw []byte
	if err := st.Pool().QueryRow(context.Background(), `select national_id_sealed from party where party_id = $1::uuid`, p.PartyId).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("8001010012345")) {
		t.Fatal("national ID stored in plaintext")
	}
	if evs := statusEvents(t, st); len(evs) != 1 || evs[0].ToStatus != "PARTY_STATUS_PROSPECT" {
		t.Fatalf("creation must emit one status event, got %+v", evs)
	}
}

func TestCreateIsIdempotent(t *testing.T) {
	s, st := newService(t)
	first := mustCreate(t, s, company("c1", "Forge Pilot (Pty) Ltd", "2026/123456/07"))
	again := mustCreate(t, s, company("c1", "Forge Pilot (Pty) Ltd", "2026/123456/07"))
	if first.PartyId != again.PartyId {
		t.Fatal("replay must return the same party")
	}
	if len(statusEvents(t, st)) != 1 {
		t.Fatal("replay must not emit a second event")
	}
	_, err := s.CreateParty(context.Background(), company("c1", "Other name", "2026/123456/07"))
	if codeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("same key, different request: want AlreadyExists, got %v", err)
	}
}

func TestDuplicatesAreRefused(t *testing.T) {
	s, _ := newService(t)
	mustCreate(t, s, person("p1", "Thandi Dube", "8001010012345"))
	_, err := s.CreateParty(context.Background(), person("p2", "Another Name", "8001 0100 12345"))
	if codeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("same national ID under another key: want AlreadyExists, got %v", err)
	}
	mustCreate(t, s, company("c1", "Forge Pilot", "2026/1"))
	_, err = s.CreateParty(context.Background(), company("c2", "Different", "2026/1"))
	if codeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("same registration number: want AlreadyExists, got %v", err)
	}
}

func TestLifecycleFollowsTheTable(t *testing.T) {
	s, st := newService(t)
	ctx := context.Background()
	p := mustCreate(t, s, company("c1", "Forge Pilot", "2026/2"))

	setStatus := func(to partyv1.PartyStatus) error {
		_, err := s.SetStatus(ctx, connect.NewRequest(&partyv1.SetStatusRequest{PartyId: p.PartyId, ToStatus: to, Actor: "test"}))
		return err
	}
	if codeOf(setStatus(partyv1.PartyStatus_PARTY_STATUS_ACTIVE)) != connect.CodeFailedPrecondition {
		t.Fatal("PROSPECT -> ACTIVE skips KYC and must be refused")
	}
	if err := setStatus(partyv1.PartyStatus_PARTY_STATUS_KYC_PENDING); err != nil {
		t.Fatal(err)
	}
	if err := setStatus(partyv1.PartyStatus_PARTY_STATUS_ACTIVE); err != nil {
		t.Fatal(err)
	}
	if err := setStatus(partyv1.PartyStatus_PARTY_STATUS_CLOSED); err != nil {
		t.Fatal(err)
	}
	if codeOf(setStatus(partyv1.PartyStatus_PARTY_STATUS_ACTIVE)) != connect.CodeFailedPrecondition {
		t.Fatal("closed is terminal")
	}
	// creation + three applied transitions, and nothing for the two refused ones
	if n := len(statusEvents(t, st)); n != 4 {
		t.Fatalf("status events = %d, want 4", n)
	}
}

func TestFounderAndCompanyStaySeparate(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	co := mustCreate(t, s, company("c1", "Forge Pilot", "2026/3"))
	pe := mustCreate(t, s, person("p1", "Thandi Dube", "8001010012345"))

	addRel := func(key string, from, to string, rt partyv1.RelationshipType, bps uint32) (*partyv1.Relationship, error) {
		res, err := s.AddRelationship(ctx, connect.NewRequest(&partyv1.AddRelationshipRequest{
			IdempotencyKey: key, FromPartyId: from, ToPartyId: to, Type: rt, PercentageBps: bps, Actor: "test",
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg.Relationship, nil
	}

	if _, err := addRel("r0", co.PartyId, pe.PartyId, partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR, 0); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a company cannot be a director: %v", err)
	}
	if _, err := addRel("r0b", pe.PartyId, pe.PartyId, partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR, 0); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("self relationship must be refused: %v", err)
	}
	dir, err := addRel("r1", pe.PartyId, co.PartyId, partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR, 0)
	if err != nil {
		t.Fatalf("person directs company: %v", err)
	}
	replay, err := addRel("r1", pe.PartyId, co.PartyId, partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR, 0)
	if err != nil || replay.RelationshipId != dir.RelationshipId {
		t.Fatalf("replay: %v %v", err, replay)
	}
	if _, err := addRel("r2", pe.PartyId, co.PartyId, partyv1.RelationshipType_RELATIONSHIP_TYPE_DIRECTOR, 0); codeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("second active director row: want AlreadyExists, got %v", err)
	}
	if _, err := addRel("r3", pe.PartyId, co.PartyId, partyv1.RelationshipType_RELATIONSHIP_TYPE_UBO, 0); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UBO needs a share: %v", err)
	}
	if _, err := addRel("r4", pe.PartyId, co.PartyId, partyv1.RelationshipType_RELATIONSHIP_TYPE_UBO, 2600); err != nil {
		t.Fatalf("UBO with share: %v", err)
	}

	list, err := s.ListRelationships(ctx, connect.NewRequest(&partyv1.ListRelationshipsRequest{PartyId: co.PartyId}))
	if err != nil || len(list.Msg.Relationships) != 2 {
		t.Fatalf("company relationships = %v, %v", list, err)
	}
}

func TestSetTierValidatesLabel(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	p := mustCreate(t, s, company("c1", "Forge Pilot", "2026/4"))
	if _, err := s.SetTier(ctx, connect.NewRequest(&partyv1.SetTierRequest{PartyId: p.PartyId, Tier: "platinum"})); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("unknown tier: %v", err)
	}
	res, err := s.SetTier(ctx, connect.NewRequest(&partyv1.SetTierRequest{PartyId: p.PartyId, Tier: "standard"}))
	if err != nil || res.Msg.Party.Tier != "standard" {
		t.Fatalf("set tier: %v %v", res, err)
	}
}

func TestInputValidation(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.CreateParty(ctx, person("p1", "X", "123")); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("short national ID: %v", err)
	}
	if _, err := s.CreateParty(ctx, connect.NewRequest(&partyv1.CreatePartyRequest{IdempotencyKey: "z", Type: partyv1.PartyType_PARTY_TYPE_FUND, LegalName: "F", Country: "za"})); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("lower-case country: %v", err)
	}
	if _, err := s.GetParty(ctx, connect.NewRequest(&partyv1.GetPartyRequest{PartyId: "not-a-uuid"})); codeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("bad id: %v", err)
	}
	if _, err := s.GetParty(ctx, connect.NewRequest(&partyv1.GetPartyRequest{PartyId: "11111111-1111-4111-8111-111111111111"})); codeOf(err) != connect.CodeNotFound {
		t.Fatalf("missing party: %v", err)
	}
}
