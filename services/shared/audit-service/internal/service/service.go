// Package service is the ConnectRPC surface of the audit log. The actor of every entry is the authenticated caller,
// never a field the caller supplies, so a service can only write under its own identity.
package service

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"connectrpc.com/connect"

	auditv1 "github.com/khayaai/forge-mutual/pkg/api/forge/audit/v1"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/chain"
	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/store"
)

// Service implements auditv1connect.AuditServiceHandler.
type Service struct {
	Store *store.Store
}

// Append writes one entry under the caller's identity.
func (s *Service) Append(ctx context.Context, req *connect.Request[auditv1.AppendRequest]) (*connect.Response[auditv1.AppendResponse], error) {
	actor := callerauth.Caller(ctx, req.Header())
	e, err := s.Store.Append(ctx, chain.Input{
		Actor: actor, Action: req.Msg.Action, Entity: req.Msg.Entity, Payload: []byte(req.Msg.PayloadJson),
	})
	if err != nil {
		if errors.Is(err, chain.ErrInvalidInput) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&auditv1.AppendResponse{Seq: e.Seq, Hash: hex.EncodeToString(e.Hash[:])}), nil
}

// Query pages the log, optionally for one entity, in sequence order.
func (s *Service) Query(ctx context.Context, req *connect.Request[auditv1.QueryRequest]) (*connect.Response[auditv1.QueryResponse], error) {
	if req.Msg.Limit < 1 || req.Msg.Limit > 500 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("limit must be between 1 and 500"))
	}
	limit := int(req.Msg.Limit)
	rows, err := s.Store.Range(ctx, req.Msg.Entity, req.Msg.AfterSeq, limit+1)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &auditv1.QueryResponse{}
	if len(rows) > limit {
		out.NextAfterSeq = rows[limit-1].Seq
		rows = rows[:limit]
	}
	for _, e := range rows {
		out.Entries = append(out.Entries, &auditv1.Entry{
			Seq: e.Seq, At: e.At.UTC().Format(time.RFC3339Nano), Actor: e.Actor, Action: e.Action, Entity: e.Entity,
			PayloadJson: string(e.Payload), PayloadHash: hex.EncodeToString(e.PayloadHash[:]),
			PrevHash: hex.EncodeToString(e.PrevHash[:]), Hash: hex.EncodeToString(e.Hash[:]),
		})
	}
	return connect.NewResponse(out), nil
}

// Head returns the last sequence number and its hash.
func (s *Service) Head(ctx context.Context, _ *connect.Request[auditv1.HeadRequest]) (*connect.Response[auditv1.HeadResponse], error) {
	seq, h, err := s.Store.Head(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&auditv1.HeadResponse{Seq: seq, Hash: hex.EncodeToString(h[:])}), nil
}
