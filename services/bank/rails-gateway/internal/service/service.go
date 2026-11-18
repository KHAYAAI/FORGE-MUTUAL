// Package service exposes a rail over ConnectRPC.
package service

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	railsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1/railsv1connect"
	"github.com/khayaai/forge-mutual/services/bank/rails-gateway/internal/rail"
)

// Service implements RailService on top of a rail.
type Service struct {
	r rail.Rail
}

var _ railsv1connect.RailServiceHandler = (*Service)(nil)

// New wraps a rail.
func New(r rail.Rail) *Service { return &Service{r: r} }

// Initiate implements RailServiceHandler.
func (s *Service) Initiate(_ context.Context, req *connect.Request[railsv1.InitiateRequest]) (*connect.Response[railsv1.InitiateResponse], error) {
	m := req.Msg
	if m.InstructionId == "" || len(m.InstructionId) > 64 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("instruction_id must be 1 to 64 characters"))
	}
	res, err := s.r.Initiate(rail.Instruction{
		ID: m.InstructionId, Rail: m.Rail, AmountMinor: m.AmountMinor, Asset: m.Asset,
		BeneficiaryName: m.BeneficiaryName, AccountNumber: m.AccountNumber, BankCode: m.BankCode, Scenario: m.Scenario,
	})
	switch {
	case errors.Is(err, rail.ErrConflict):
		return nil, connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, rail.ErrScenarioRefused), errors.Is(err, rail.ErrBadScenario):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, errors.New("rail failure"))
	}
	return connect.NewResponse(&railsv1.InitiateResponse{Status: res.Status, Reference: res.Reference}), nil
}

// Status implements RailServiceHandler.
func (s *Service) Status(_ context.Context, req *connect.Request[railsv1.StatusRequest]) (*connect.Response[railsv1.StatusResponse], error) {
	res, err := s.r.Status(req.Msg.InstructionId)
	if errors.Is(err, rail.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("rail failure"))
	}
	return connect.NewResponse(&railsv1.StatusResponse{Status: res.Status, Reference: res.Reference}), nil
}
