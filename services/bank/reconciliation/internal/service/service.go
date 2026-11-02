// Package service is the ConnectRPC surface of the reconciliation service. The gate is what payments reads before
// moving money (invariant 9). Investigation records findings and never changes the gate.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	"github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/recon"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/runner"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/store"
)

// Service implements reconv1connect.ReconServiceHandler.
type Service struct {
	Store  *store.Store
	Runner *runner.Runner
	Assets []string
	MaxAge time.Duration
	Now    func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// GetGate reports whether payments may proceed. Any failure to read the state is returned as an error, which the
// caller must treat as closed.
func (s *Service) GetGate(ctx context.Context, _ *connect.Request[reconv1.GetGateRequest]) (*connect.Response[reconv1.GetGateResponse], error) {
	now := s.now()
	g, err := s.Store.Gate(ctx, s.Assets, now, s.MaxAge)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&reconv1.GetGateResponse{
		BreakActive: !g.Open,
		Reasons:     g.Reasons,
		AsOf:        now.UTC().Format(time.RFC3339Nano),
	}), nil
}

// ListBreaks returns breaks, newest first.
func (s *Service) ListBreaks(ctx context.Context, req *connect.Request[reconv1.ListBreaksRequest]) (*connect.Response[reconv1.ListBreaksResponse], error) {
	list, err := s.Store.ListBreaks(ctx, req.Msg.OpenOnly)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &reconv1.ListBreaksResponse{}
	for _, b := range list {
		out.Breaks = append(out.Breaks, toProto(b))
	}
	return connect.NewResponse(out), nil
}

// InvestigateBreak records who looked at a break and what they found. The actor is the authenticated caller.
func (s *Service) InvestigateBreak(ctx context.Context, req *connect.Request[reconv1.InvestigateBreakRequest]) (*connect.Response[reconv1.InvestigateBreakResponse], error) {
	actor := callerauth.Caller(ctx, req.Header())
	b, err := s.Store.Investigate(ctx, req.Msg.BreakId, actor, req.Msg.Note)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, connect.NewError(connect.CodeNotFound, err)
	case err != nil && (actor == "" || req.Msg.Note == ""):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&reconv1.InvestigateBreakResponse{Break: toProto(b)}), nil
}

// RunRecon runs one asset now. It is for operators and tests; the scheduled loop runs the same path.
func (s *Service) RunRecon(ctx context.Context, req *connect.Request[reconv1.RunReconRequest]) (*connect.Response[reconv1.RunReconResponse], error) {
	if _, ok := recon.ByAsset(req.Msg.Asset); !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("asset %q is not monitored", req.Msg.Asset))
	}
	out, err := s.Runner.Run(ctx, req.Msg.Asset)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	resp := &reconv1.RunReconResponse{RunId: out.RunID, Outcome: out.Outcome}
	for _, c := range out.Checks {
		resp.Checks = append(resp.Checks, &reconv1.CheckResult{
			Check: c.Check, Asset: out.Asset, DifferenceMinor: fmt.Sprint(c.Difference), Matched: c.Matched, Detail: c.Detail,
		})
	}
	return connect.NewResponse(resp), nil
}

func toProto(b store.Break) *reconv1.Break {
	p := &reconv1.Break{
		BreakId:           b.BreakID,
		Asset:             b.Asset,
		Check:             b.Check,
		DifferenceMinor:   fmt.Sprint(b.DifferenceMinor),
		Detail:            b.Detail,
		Status:            b.Status,
		OpenedAt:          b.OpenedAt.UTC().Format(time.RFC3339Nano),
		LastSeenAt:        b.LastSeenAt.UTC().Format(time.RFC3339Nano),
		InvestigationNote: b.InvestigationNote,
		InvestigatedBy:    b.InvestigatedBy,
	}
	if b.ClearedAt != nil {
		p.ClearedAt = b.ClearedAt.UTC().Format(time.RFC3339Nano)
	}
	if b.InvestigatedAt != nil {
		p.InvestigatedAt = b.InvestigatedAt.UTC().Format(time.RFC3339Nano)
	}
	return p
}
