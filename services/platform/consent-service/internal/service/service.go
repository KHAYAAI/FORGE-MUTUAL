// Package service implements the consent API. A grant or revocation writes its event to the outbox in
// the same transaction (ARCHITECTURE.md Appendix C: forge.plat.consent.granted.v1 and .revoked.v1).
package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"regexp"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	consentv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1/consentv1connect"
	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/platform/consent-service/internal/store"
)

// Event topics (ARCHITECTURE.md Appendix C). Both carry eventsv1.ConsentChanged.
const (
	TopicGranted = "forge.plat.consent.granted.v1"
	TopicRevoked = "forge.plat.consent.revoked.v1"
)

var (
	purposeRe = regexp.MustCompile(`^[a-z][a-z0-9_.]{2,63}$`)
	scopeRe   = regexp.MustCompile(`^[a-z][a-z0-9_.:-]{1,63}$`)
)

const maxList = 16

// Service implements ConsentService.
type Service struct{ st *store.Store }

var _ consentv1connect.ConsentServiceHandler = (*Service)(nil)

// New returns the service.
func New(st *store.Store) *Service { return &Service{st: st} }

// Grant implements ConsentServiceHandler. A repeated key with the same request returns the consent it
// created; the same key with a different request is refused.
func (s *Service) Grant(ctx context.Context, req *connect.Request[consentv1.GrantRequest]) (*connect.Response[consentv1.GrantResponse], error) {
	m := req.Msg
	if err := validateKey(m.IdempotencyKey); err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(m.PartyId); err != nil {
		return nil, invalid("party_id must be a canonical UUID")
	}
	if !purposeRe.MatchString(m.Purpose) {
		return nil, invalid("purpose must be 3 to 64 lower-case letters, digits, dots or underscores")
	}
	if err := validateList("scopes", m.Scopes, scopeRe); err != nil {
		return nil, err
	}
	if err := validateList("data_sources", m.DataSources, scopeRe); err != nil {
		return nil, err
	}
	if ev := strings.TrimSpace(m.EvidenceRef); ev == "" || len(ev) > 256 {
		return nil, invalid("evidence_ref is required, up to 256 characters")
	}
	if actor := strings.TrimSpace(m.Actor); actor == "" || len(actor) > 64 {
		return nil, invalid("actor is required, up to 64 characters")
	}
	now := time.Now().UTC()
	var expires *time.Time
	if m.ExpiresAt != nil {
		t := m.ExpiresAt.AsTime().UTC()
		if !t.After(now) {
			return nil, invalid("expires_at must be in the future")
		}
		expires = &t
	}
	hash, err := requestHash("consent.grant", m)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var out store.Consent
	err = s.st.WithTx(ctx, func(tx pgx.Tx) error {
		prev, resourceID, found, err := store.GetIdempotency(ctx, tx, "consent.grant", m.IdempotencyKey)
		if err != nil {
			return err
		}
		if found {
			if string(prev) != string(hash) {
				return errIdempotencyConflict
			}
			out, err = store.Get(ctx, tx, resourceID)
			return err
		}
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}
		out = store.Consent{
			ID: id, PartyID: m.PartyId, Purpose: m.Purpose, Scopes: m.Scopes, DataSources: m.DataSources,
			GrantedAt: now, ExpiresAt: expires, EvidenceRef: m.EvidenceRef, GrantedBy: m.Actor,
		}
		if err := store.Insert(ctx, tx, out); err != nil {
			return err
		}
		if err := store.PutIdempotency(ctx, tx, "consent.grant", m.IdempotencyKey, hash, id); err != nil {
			return err
		}
		return appendChange(ctx, tx, TopicGranted, out.ID, out.PartyID, out.Purpose, "GRANTED", m.Actor)
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&consentv1.GrantResponse{Consent: toProto(out)}), nil
}

// Revoke implements ConsentServiceHandler. Revoking a consent that is already revoked changes nothing
// and writes no event.
func (s *Service) Revoke(ctx context.Context, req *connect.Request[consentv1.RevokeRequest]) (*connect.Response[consentv1.RevokeResponse], error) {
	m := req.Msg
	if _, err := uuid.Parse(m.ConsentId); err != nil {
		return nil, invalid("consent_id must be a canonical UUID")
	}
	if strings.TrimSpace(m.Actor) == "" {
		return nil, invalid("actor is required")
	}
	var out store.Consent
	err := s.st.WithTx(ctx, func(tx pgx.Tx) error {
		c, err := store.Lock(ctx, tx, m.ConsentId)
		if err != nil {
			return err
		}
		if c.RevokedAt == nil {
			if err := store.Revoke(ctx, tx, m.ConsentId); err != nil {
				return err
			}
			if err := appendChange(ctx, tx, TopicRevoked, c.ID, c.PartyID, c.Purpose, "REVOKED", m.Actor); err != nil {
				return err
			}
		}
		out, err = store.Get(ctx, tx, m.ConsentId)
		return err
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&consentv1.RevokeResponse{Consent: toProto(out)}), nil
}

// Check implements ConsentServiceHandler. It reads the consents as they are now, so a revocation
// is effective on the next call.
func (s *Service) Check(ctx context.Context, req *connect.Request[consentv1.CheckRequest]) (*connect.Response[consentv1.CheckResponse], error) {
	m := req.Msg
	if _, err := uuid.Parse(m.PartyId); err != nil {
		return nil, invalid("party_id must be a canonical UUID")
	}
	if !purposeRe.MatchString(m.Purpose) || !scopeRe.MatchString(m.Scope) {
		return nil, invalid("purpose and scope are required and must be well formed")
	}
	c, err := store.FindValid(ctx, s.st.Pool(), m.PartyId, m.Purpose, m.Scope)
	if errors.Is(err, store.ErrNotFound) {
		return connect.NewResponse(&consentv1.CheckResponse{Allowed: false, Reason: "no_valid_consent"}), nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&consentv1.CheckResponse{Allowed: true, Reason: "consent_granted", ConsentId: c.ID}), nil
}

// List implements ConsentServiceHandler.
func (s *Service) List(ctx context.Context, req *connect.Request[consentv1.ListRequest]) (*connect.Response[consentv1.ListResponse], error) {
	if _, err := uuid.Parse(req.Msg.PartyId); err != nil {
		return nil, invalid("party_id must be a canonical UUID")
	}
	cs, err := store.ListForParty(ctx, s.st.Pool(), req.Msg.PartyId)
	if err != nil {
		return nil, mapErr(err)
	}
	out := &consentv1.ListResponse{}
	for _, c := range cs {
		out.Consents = append(out.Consents, toProto(c))
	}
	return connect.NewResponse(out), nil
}

// ---- helpers ----

var errIdempotencyConflict = errors.New("idempotency key reused with a different request")

func appendChange(ctx context.Context, tx pgx.Tx, topic, consentID, partyID, purpose, action, actor string) error {
	payload, err := proto.Marshal(&eventsv1.ConsentChanged{
		ConsentId: consentID, PartyId: partyID, Purpose: purpose, Action: action, Actor: actor,
	})
	if err != nil {
		return err
	}
	eventID, err := outbox.NewEventID()
	if err != nil {
		return err
	}
	return outbox.Append(ctx, tx, outbox.Event{EventID: eventID, Topic: topic, Key: []byte(partyID), Payload: payload})
}

func validateKey(k string) error {
	if k == "" || len(k) > 128 {
		return invalid("idempotency_key must be 1 to 128 characters")
	}
	return nil
}

func validateList(field string, xs []string, re *regexp.Regexp) error {
	if len(xs) == 0 || len(xs) > maxList {
		return invalid(field + " must hold 1 to 16 values")
	}
	seen := map[string]bool{}
	for _, x := range xs {
		if !re.MatchString(x) {
			return invalid(field + " contains a malformed value")
		}
		if seen[x] {
			return invalid(field + " contains a duplicate")
		}
		seen[x] = true
	}
	return nil
}

func invalid(msg string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(msg)) }

func mapErr(err error) error {
	var conn *connect.Error
	if errors.As(err, &conn) {
		return conn
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("consent not found"))
	case errors.Is(err, store.ErrDuplicate), errors.Is(err, errIdempotencyConflict):
		return connect.NewError(connect.CodeAlreadyExists, errIdempotencyConflict)
	}
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

func toProto(c store.Consent) *consentv1.Consent {
	out := &consentv1.Consent{
		ConsentId:   c.ID,
		PartyId:     c.PartyID,
		Purpose:     c.Purpose,
		Scopes:      c.Scopes,
		DataSources: c.DataSources,
		GrantedAt:   timestamppb.New(c.GrantedAt),
		EvidenceRef: c.EvidenceRef,
	}
	if c.ExpiresAt != nil {
		out.ExpiresAt = timestamppb.New(*c.ExpiresAt)
	}
	if c.RevokedAt != nil {
		out.RevokedAt = timestamppb.New(*c.RevokedAt)
	}
	return out
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
