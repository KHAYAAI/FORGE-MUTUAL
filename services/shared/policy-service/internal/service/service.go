// Package service exposes the policy decision API. Every decision is written to the audit
// outbox in the same transaction. If that write fails, no decision is returned.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	policyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1/policyv1connect"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/services/shared/policy-service/internal/engine"
)

// DecisionTopic is the audit topic for every policy decision (ARCHITECTURE.md Appendix C).
const DecisionTopic = "forge.shared.policy.decision.v1"

// Service implements PolicyService.
type Service struct {
	eng  atomic.Pointer[engine.Engine] // nil until a verified bundle is loaded
	pool *pgxpool.Pool
}

var _ policyv1connect.PolicyServiceHandler = (*Service)(nil)

// New returns a service. eng may be nil, in which case every decision is a denial.
func New(pool *pgxpool.Pool, eng *engine.Engine) *Service {
	s := &Service{pool: pool}
	s.eng.Store(eng)
	return s
}

// Swap replaces the bundle engine. Used when a new signed bundle is loaded.
func (s *Service) Swap(eng *engine.Engine) { s.eng.Store(eng) }

// Evaluate implements PolicyServiceHandler.
func (s *Service) Evaluate(ctx context.Context, req *connect.Request[policyv1.EvaluateRequest]) (*connect.Response[policyv1.EvaluateResponse], error) {
	m := req.Msg
	if !engine.Known(m.DecisionPoint) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown decision point %q", m.DecisionPoint))
	}
	if m.EntityCode == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("entity_code required"))
	}
	input := m.Input.AsMap()
	canonical, err := json.Marshal(input) // map keys are sorted, so the digest is stable
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	digest := sha256.Sum256(canonical)
	decisionID, err := outbox.NewEventID()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	version := "none"
	dec := engine.Denied("policy_bundle_unavailable")
	if eng := s.eng.Load(); eng != nil {
		version = eng.Version()
		d, err := eng.Evaluate(ctx, m.DecisionPoint, input)
		if err != nil {
			dec = engine.Denied("evaluation_error") // unreadable is a denial, never an allow
		} else {
			dec = d
		}
	}

	event := &eventsv1.PolicyDecision{
		DecisionId:    decisionID,
		DecisionPoint: m.DecisionPoint,
		EntityCode:    m.EntityCode,
		RequestId:     m.RequestId,
		BundleVersion: version,
		Allow:         dec.Allow,
		Stepup:        dec.StepUp,
		Reason:        dec.Reason,
		InputSha256:   digest[:],
	}
	payload, err := proto.Marshal(event)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := s.audit(ctx, outbox.Event{EventID: decisionID, Topic: DecisionTopic, Key: []byte(decisionID), Payload: payload}); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("decision not audited: %w", err))
	}
	return connect.NewResponse(&policyv1.EvaluateResponse{
		Allow:         dec.Allow,
		Stepup:        dec.StepUp,
		Reason:        dec.Reason,
		BundleVersion: version,
		DecisionId:    decisionID,
	}), nil
}

func (s *Service) audit(ctx context.Context, e outbox.Event) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	if err := outbox.Append(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
