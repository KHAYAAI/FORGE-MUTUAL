// Package service implements the party API. Every status change is written to the outbox in the
// same transaction as the change (ARCHITECTURE.md Appendix C: forge.plat.party.status_changed.v1).
package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/platform/party-service/internal/lifecycle"
	"github.com/khayaai/forge-mutual/services/platform/party-service/internal/sealer"
	"github.com/khayaai/forge-mutual/services/platform/party-service/internal/store"
)

// TopicStatusChanged is the party status event (ARCHITECTURE.md Appendix C).
const TopicStatusChanged = "forge.plat.party.status_changed.v1"

// tiers are the labels policy-service knows about (policy/tiers.json).
var tiers = map[string]bool{"standard": true, "enhanced": true}

var (
	countryRe  = regexp.MustCompile(`^[A-Z]{2}$`)
	nationalRe = regexp.MustCompile(`^[A-Za-z0-9]{6,20}$`)
)

// Domain errors raised inside transactions and mapped to API codes by mapErr.
var (
	errIdempotencyConflict = errors.New("idempotency key reused with a different request")
)

type errRelationship struct{ msg string }

func (e errRelationship) Error() string { return e.msg }

type errTransition struct{ from, to partyv1.PartyStatus }

func (e errTransition) Error() string {
	return fmt.Sprintf("transition %s -> %s is not allowed", e.from, e.to)
}

// Service implements PartyService.
type Service struct {
	st  *store.Store
	sl  *sealer.Sealer
	log *slog.Logger
}

var _ partyv1connect.PartyServiceHandler = (*Service)(nil)

// New returns the service.
func New(st *store.Store, sl *sealer.Sealer, log *slog.Logger) *Service {
	return &Service{st: st, sl: sl, log: log}
}

// CreateParty implements PartyServiceHandler. A repeated key with the same request returns the party
// it created; the same key with a different request is refused.
func (s *Service) CreateParty(ctx context.Context, req *connect.Request[partyv1.CreatePartyRequest]) (*connect.Response[partyv1.CreatePartyResponse], error) {
	m := req.Msg
	if err := requireKey(m.IdempotencyKey); err != nil {
		return nil, err
	}
	if err := validateName(m.LegalName); err != nil {
		return nil, err
	}
	if !countryRe.MatchString(m.Country) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("country must be an ISO 3166-1 alpha-2 code"))
	}
	hash, err := requestHash("party.create", m)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var out store.Party
	err = s.st.WithTx(ctx, func(tx pgx.Tx) error {
		prevHash, resourceID, found, err := store.GetIdempotency(ctx, tx, "party.create", m.IdempotencyKey)
		if err != nil {
			return err
		}
		if found {
			if string(prevHash) != string(hash) {
				return errIdempotencyConflict
			}
			out, err = store.GetParty(ctx, tx, resourceID)
			return err
		}
		p, err := s.newParty(m)
		if err != nil {
			return err
		}
		if err := store.InsertParty(ctx, tx, p); err != nil {
			return err
		}
		if err := store.PutIdempotency(ctx, tx, "party.create", m.IdempotencyKey, hash, p.ID); err != nil {
			return err
		}
		if err := appendStatus(ctx, tx, p.ID, "", p.Status, "created", m.Actor); err != nil {
			return err
		}
		out = p
		return nil
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&partyv1.CreatePartyResponse{Party: toProto(out)}), nil
}

// GetParty implements PartyServiceHandler.
func (s *Service) GetParty(ctx context.Context, req *connect.Request[partyv1.GetPartyRequest]) (*connect.Response[partyv1.GetPartyResponse], error) {
	if err := requireUUID(req.Msg.PartyId); err != nil {
		return nil, err
	}
	p, err := store.GetParty(ctx, s.st.Pool(), req.Msg.PartyId)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&partyv1.GetPartyResponse{Party: toProto(p)}), nil
}

// AddRelationship implements PartyServiceHandler. lifecycle.CheckRelationship keeps founders and
// companies separate (invariant 12).
func (s *Service) AddRelationship(ctx context.Context, req *connect.Request[partyv1.AddRelationshipRequest]) (*connect.Response[partyv1.AddRelationshipResponse], error) {
	m := req.Msg
	if err := requireKey(m.IdempotencyKey); err != nil {
		return nil, err
	}
	if err := requireUUID(m.FromPartyId); err != nil {
		return nil, err
	}
	if err := requireUUID(m.ToPartyId); err != nil {
		return nil, err
	}
	if m.FromPartyId == m.ToPartyId {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a party cannot relate to itself"))
	}
	if m.PercentageBps > 10000 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("percentage_bps must be 0 to 10000"))
	}
	hash, err := requestHash("relationship.add", m)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var out store.Relationship
	err = s.st.WithTx(ctx, func(tx pgx.Tx) error {
		prevHash, resourceID, found, err := store.GetIdempotency(ctx, tx, "relationship.add", m.IdempotencyKey)
		if err != nil {
			return err
		}
		if found {
			if string(prevHash) != string(hash) {
				return errIdempotencyConflict
			}
			out, err = store.GetRelationship(ctx, tx, resourceID)
			return err
		}
		from, err := store.LockParty(ctx, tx, m.FromPartyId)
		if err != nil {
			return err
		}
		to, err := store.LockParty(ctx, tx, m.ToPartyId)
		if err != nil {
			return err
		}
		if msg := lifecycle.CheckRelationship(m.Type, partyTypeOf(from.Type), partyTypeOf(to.Type), m.PercentageBps); msg != "" {
			return errRelationship{msg}
		}
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}
		out = store.Relationship{ID: id, From: m.FromPartyId, To: m.ToPartyId, Type: m.Type.String(), Bps: m.PercentageBps, Active: true}
		if err := store.InsertRelationship(ctx, tx, out); err != nil {
			return err
		}
		return store.PutIdempotency(ctx, tx, "relationship.add", m.IdempotencyKey, hash, id)
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&partyv1.AddRelationshipResponse{Relationship: relationshipToProto(out)}), nil
}

// ListRelationships implements PartyServiceHandler.
func (s *Service) ListRelationships(ctx context.Context, req *connect.Request[partyv1.ListRelationshipsRequest]) (*connect.Response[partyv1.ListRelationshipsResponse], error) {
	if err := requireUUID(req.Msg.PartyId); err != nil {
		return nil, err
	}
	rels, err := store.ListRelationships(ctx, s.st.Pool(), req.Msg.PartyId)
	if err != nil {
		return nil, mapErr(err)
	}
	out := &partyv1.ListRelationshipsResponse{}
	for _, r := range rels {
		out.Relationships = append(out.Relationships, relationshipToProto(r))
	}
	return connect.NewResponse(out), nil
}

// SetStatus implements PartyServiceHandler. Only transitions in lifecycle.Allowed are applied.
func (s *Service) SetStatus(ctx context.Context, req *connect.Request[partyv1.SetStatusRequest]) (*connect.Response[partyv1.SetStatusResponse], error) {
	m := req.Msg
	if err := requireUUID(m.PartyId); err != nil {
		return nil, err
	}
	to := m.ToStatus
	if to == partyv1.PartyStatus_PARTY_STATUS_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("to_status is required"))
	}
	var out store.Party
	err := s.st.WithTx(ctx, func(tx pgx.Tx) error {
		p, err := store.LockParty(ctx, tx, m.PartyId)
		if err != nil {
			return err
		}
		from := statusOf(p.Status)
		if !lifecycle.Allowed(from, to) {
			return errTransition{from, to}
		}
		if err := store.SetStatus(ctx, tx, m.PartyId, p.Status, to.String()); err != nil {
			return err
		}
		if err := appendStatus(ctx, tx, m.PartyId, p.Status, to.String(), m.Reason, m.Actor); err != nil {
			return err
		}
		p.Status = to.String()
		out = p
		return nil
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&partyv1.SetStatusResponse{Party: toProto(out)}), nil
}

// SetTier implements PartyServiceHandler.
func (s *Service) SetTier(ctx context.Context, req *connect.Request[partyv1.SetTierRequest]) (*connect.Response[partyv1.SetTierResponse], error) {
	m := req.Msg
	if err := requireUUID(m.PartyId); err != nil {
		return nil, err
	}
	if !tiers[m.Tier] {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown tier %q", m.Tier))
	}
	var out store.Party
	err := s.st.WithTx(ctx, func(tx pgx.Tx) error {
		if _, err := store.LockParty(ctx, tx, m.PartyId); err != nil {
			return err
		}
		if err := store.SetTier(ctx, tx, m.PartyId, m.Tier); err != nil {
			return err
		}
		p, err := store.GetParty(ctx, tx, m.PartyId)
		out = p
		return err
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&partyv1.SetTierResponse{Party: toProto(out)}), nil
}

// ---- helpers ----

// newParty validates the type-specific fields and builds a PROSPECT party.
func (s *Service) newParty(m *partyv1.CreatePartyRequest) (store.Party, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return store.Party{}, err
	}
	p := store.Party{
		ID:        id,
		Type:      m.Type.String(),
		Status:    partyv1.PartyStatus_PARTY_STATUS_PROSPECT.String(),
		LegalName: strings.TrimSpace(m.LegalName),
		Country:   m.Country,
	}
	switch m.Type {
	case partyv1.PartyType_PARTY_TYPE_PERSON:
		if !nationalRe.MatchString(strings.ReplaceAll(m.NationalId, " ", "")) {
			return store.Party{}, connect.NewError(connect.CodeInvalidArgument, errors.New("national_id must be 6 to 20 letters or digits"))
		}
		sealed, err := s.sl.Seal(m.NationalId)
		if err != nil {
			return store.Party{}, err
		}
		p.SealedNationalID = sealed
		p.NationalIDIndex = s.sl.Index(m.NationalId)
		p.NationalIDMasked = sealer.Mask(m.NationalId)
	case partyv1.PartyType_PARTY_TYPE_COMPANY:
		rn := strings.TrimSpace(m.RegistrationNumber)
		if rn == "" || len(rn) > 32 {
			return store.Party{}, connect.NewError(connect.CodeInvalidArgument, errors.New("registration_number is required, up to 32 characters"))
		}
		p.RegistrationNumber = rn
	case partyv1.PartyType_PARTY_TYPE_FUND:
	default:
		return store.Party{}, connect.NewError(connect.CodeInvalidArgument, errors.New("type is required"))
	}
	return p, nil
}

func appendStatus(ctx context.Context, tx pgx.Tx, partyID, from, to, reason, actor string) error {
	payload, err := proto.Marshal(&eventsv1.PartyStatusChanged{
		PartyId: partyID, FromStatus: from, ToStatus: to, Reason: reason, Actor: actor,
	})
	if err != nil {
		return err
	}
	eventID, err := outbox.NewEventID()
	if err != nil {
		return err
	}
	return outbox.Append(ctx, tx, outbox.Event{EventID: eventID, Topic: TopicStatusChanged, Key: []byte(partyID), Payload: payload})
}

// mapErr turns store and domain errors into API errors. Database detail is not returned to callers.
func mapErr(err error) error {
	var conn *connect.Error
	if errors.As(err, &conn) {
		return conn
	}
	var rel errRelationship
	var tr errTransition
	switch {
	case errors.Is(err, store.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("party not found"))
	case errors.Is(err, store.ErrDuplicate):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("an identical party or relationship already exists"))
	case errors.Is(err, store.ErrStale):
		return connect.NewError(connect.CodeAborted, errors.New("party changed concurrently; retry"))
	case errors.Is(err, errIdempotencyConflict):
		return connect.NewError(connect.CodeAlreadyExists, errIdempotencyConflict)
	case errors.As(err, &rel):
		return connect.NewError(connect.CodeInvalidArgument, errors.New(rel.msg))
	case errors.As(err, &tr):
		return connect.NewError(connect.CodeFailedPrecondition, tr)
	}
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

func toProto(p store.Party) *partyv1.Party {
	return &partyv1.Party{
		PartyId:            p.ID,
		Type:               partyTypeOf(p.Type),
		Status:             statusOf(p.Status),
		LegalName:          p.LegalName,
		RegistrationNumber: p.RegistrationNumber,
		NationalIdMasked:   p.NationalIDMasked,
		Country:            p.Country,
		Tier:               p.Tier,
		CreatedAt:          timestamppb.New(p.CreatedAt),
	}
}

func relationshipToProto(r store.Relationship) *partyv1.Relationship {
	return &partyv1.Relationship{
		RelationshipId: r.ID,
		FromPartyId:    r.From,
		ToPartyId:      r.To,
		Type:           relationshipTypeOf(r.Type),
		PercentageBps:  r.Bps,
		Active:         r.Active,
	}
}

func partyTypeOf(name string) partyv1.PartyType {
	return partyv1.PartyType(partyv1.PartyType_value[name])
}

func statusOf(name string) partyv1.PartyStatus {
	return partyv1.PartyStatus(partyv1.PartyStatus_value[name])
}

func relationshipTypeOf(name string) partyv1.RelationshipType {
	return partyv1.RelationshipType(partyv1.RelationshipType_value[name])
}

func requireKey(k string) error {
	if k == "" || len(k) > 128 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key must be 1 to 128 characters"))
	}
	return nil
}

func requireUUID(s string) error {
	if _, err := uuid.Parse(s); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("party id must be a canonical UUID"))
	}
	return nil
}

func validateName(s string) error {
	n := len(strings.TrimSpace(s))
	if n == 0 || n > 200 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("legal_name must be 1 to 200 characters"))
	}
	return nil
}

func requestHash(method string, m proto.Message) ([]byte, error) {
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte{0})
	h.Write(b)
	return h.Sum(nil), nil
}
