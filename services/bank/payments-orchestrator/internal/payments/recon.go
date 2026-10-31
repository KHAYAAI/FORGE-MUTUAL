package payments

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	reconv1 "github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1/reconv1connect"
)

// ReconStatus reports whether a sponsor reconciliation break is open (ARCHITECTURE.md 13). Payments above the
// policy threshold are held while one is.
type ReconStatus interface {
	Breaking(ctx context.Context) (bool, error)
}

// ConnectRecon reads the gate from the reconciliation service. An unreachable service is an error, never "no
// break": the activity fails and retries, so no money moves while the gate is unknown.
type ConnectRecon struct {
	Client reconv1connect.ReconServiceClient
}

// Breaking implements ReconStatus.
func (c ConnectRecon) Breaking(ctx context.Context) (bool, error) {
	resp, err := c.Client.GetGate(ctx, connect.NewRequest(&reconv1.GetGateRequest{}))
	if err != nil {
		return false, fmt.Errorf("reconciliation gate unavailable: %w", err)
	}
	return resp.Msg.BreakActive, nil
}

// StaticRecon is a fixed reconciliation status for tests that run the activities in-process. The binary never uses
// it.
type StaticRecon struct {
	Break bool
}

// Breaking implements ReconStatus.
func (s StaticRecon) Breaking(context.Context) (bool, error) { return s.Break, nil }
