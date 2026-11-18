// Package inflight reports the money in flight for one asset, for reconciliation to read as its two in-flight terms.
package inflight

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	railsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/inbound"
)

// Service implements paymentsv1connect.InFlightServiceHandler.
type Service struct {
	Pool    *pgxpool.Pool
	Inbound *inbound.Worker
}

// GetInFlight returns the inbound and outbound in-flight totals for an asset.
//
// Inbound is the sum of credits the bank has reported that are not yet booked (PENDING in the inbound store). Outbound
// is the sum of payments whose rail reports SETTLED while the payment is not COMPLETED: the sponsor has debited the
// pool, and the clearing leg has not yet been posted. A payment that needs repair stays in this total until it is
// repaired, which is correct: the money has left the bank and the books still show it.
func (s *Service) GetInFlight(ctx context.Context, req *connect.Request[paymentsv1.GetInFlightRequest]) (*connect.Response[paymentsv1.GetInFlightResponse], error) {
	asset := req.Msg.Asset
	if asset == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("asset is required"))
	}
	inboundTotal, err := s.Inbound.InFlight(ctx, asset)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("inbound in flight: %w", err))
	}
	var outbound int64
	err = s.Pool.QueryRow(ctx, `
		select coalesce(sum(amount_minor), 0) from payment
		where asset = $1 and rail_status = $2 and state <> 'COMPLETED'`,
		asset, railsv1.RailStatus_RAIL_STATUS_SETTLED.String()).Scan(&outbound)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("outbound in flight: %w", err))
	}
	return connect.NewResponse(&paymentsv1.GetInFlightResponse{
		InboundMinor:  fmt.Sprint(inboundTotal),
		OutboundMinor: fmt.Sprint(outbound),
	}), nil
}
