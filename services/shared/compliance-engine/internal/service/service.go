// Package service is the cases API: list and read compliance cases, and close one as a second person. The actor is set by
// the caller, which is an allowlisted service (the backoffice API). The rule that the opener cannot close a case is kept
// by the store, so every caller gets it.
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"

	casesv1 "github.com/khayaai/forge-mutual/pkg/api/forge/cases/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/cases/v1/casesv1connect"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/cases"
)

// Service implements CasesService.
type Service struct {
	Store *cases.Store
	Now   func() time.Time
}

var _ casesv1connect.CasesServiceHandler = (*Service)(nil)

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// ListCases implements CasesServiceHandler.
func (s *Service) ListCases(ctx context.Context, req *connect.Request[casesv1.ListCasesRequest]) (*connect.Response[casesv1.ListCasesResponse], error) {
	limit := int(req.Msg.Limit)
	if limit == 0 {
		limit = 100
	}
	if limit < 0 || limit > 500 {
		return nil, invalid("limit must be 1 to 500")
	}
	cs, err := s.Store.List(ctx, req.Msg.OpenOnly, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	now := s.now()
	out := &casesv1.ListCasesResponse{}
	for _, c := range cs {
		out.Cases = append(out.Cases, toProto(c, now))
	}
	return connect.NewResponse(out), nil
}

// GetCase implements CasesServiceHandler.
func (s *Service) GetCase(ctx context.Context, req *connect.Request[casesv1.GetCaseRequest]) (*connect.Response[casesv1.GetCaseResponse], error) {
	if _, err := uuid.Parse(req.Msg.CaseId); err != nil {
		return nil, invalid("case_id must be a canonical UUID")
	}
	c, err := s.Store.Get(ctx, req.Msg.CaseId)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&casesv1.GetCaseResponse{Case: toProto(c, s.now())}), nil
}

// CloseCase implements CasesServiceHandler.
func (s *Service) CloseCase(ctx context.Context, req *connect.Request[casesv1.CloseCaseRequest]) (*connect.Response[casesv1.CloseCaseResponse], error) {
	m := req.Msg
	if _, err := uuid.Parse(m.CaseId); err != nil {
		return nil, invalid("case_id must be a canonical UUID")
	}
	if a := strings.TrimSpace(m.Actor); a == "" || len(a) > 64 {
		return nil, invalid("actor is required, up to 64 characters")
	}
	now := s.now()
	if err := s.Store.Close(ctx, m.CaseId, strings.TrimSpace(m.Actor), m.Note, now); err != nil {
		return nil, mapErr(err)
	}
	c, err := s.Store.Get(ctx, m.CaseId)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&casesv1.CloseCaseResponse{Case: toProto(c, now)}), nil
}

func invalid(msg string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(msg)) }

func mapErr(err error) error {
	switch {
	case errors.Is(err, cases.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("case not found"))
	case errors.Is(err, cases.ErrNotOpen):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("the case is not open"))
	case errors.Is(err, cases.ErrSameClerk):
		return connect.NewError(connect.CodePermissionDenied, errors.New("the person who opened a case cannot close it"))
	case errors.Is(err, cases.ErrNoClosingNote):
		return invalid("a closing note of at least 20 characters is required")
	}
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

func toProto(c cases.Case, now time.Time) *casesv1.Case {
	out := &casesv1.Case{
		CaseId: c.CaseID, PartyId: c.PartyID, Rule: c.Rule, Severity: c.Severity, Detail: c.Detail,
		Evidence: c.Evidence, Status: c.Status, OpenedBy: c.OpenedBy,
		OpenedAt: c.OpenedAt.UTC().Format(time.RFC3339), SlaDueAt: c.SLADueAt.UTC().Format(time.RFC3339),
		Overdue:  c.Status == "OPEN" && c.SLADueAt.Before(now),
		ClosedBy: c.ClosedBy, ClosingNote: c.Closing,
	}
	if c.ClosedAt != nil {
		out.ClosedAt = c.ClosedAt.UTC().Format(time.RFC3339)
	}
	return out
}
