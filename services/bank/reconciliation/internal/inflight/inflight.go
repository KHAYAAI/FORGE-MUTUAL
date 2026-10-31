// Package inflight reads the two in-flight terms from the payments service (ARCHITECTURE.md section 13). Reconciliation
// cannot compute them itself: they live in payments and the inbound store, and reading another service's database is
// not allowed.
package inflight

import (
	"context"
	"fmt"
	"strconv"

	"connectrpc.com/connect"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/recon"
)

// Client reads in-flight money for an asset.
type Client struct {
	Client paymentsv1connect.InFlightServiceClient
}

// InFlight implements runner.InFlightSource. An unreachable source is an error. The run then records nothing, and the
// gate ages out, so unknown in-flight money can never let a run pass.
func (c Client) InFlight(ctx context.Context, asset string) (recon.InFlight, error) {
	res, err := c.Client.GetInFlight(ctx, connect.NewRequest(&paymentsv1.GetInFlightRequest{Asset: asset}))
	if err != nil {
		return recon.InFlight{}, fmt.Errorf("inflight: %s: %w", asset, err)
	}
	in, err := strconv.ParseInt(res.Msg.InboundMinor, 10, 64)
	if err != nil || in < 0 {
		return recon.InFlight{}, fmt.Errorf("inflight: inbound for %s is not a non-negative amount", asset)
	}
	out, err := strconv.ParseInt(res.Msg.OutboundMinor, 10, 64)
	if err != nil || out < 0 {
		return recon.InFlight{}, fmt.Errorf("inflight: outbound for %s is not a non-negative amount", asset)
	}
	return recon.InFlight{Inbound: in, Outbound: out}, nil
}
